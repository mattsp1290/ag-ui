package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// GreetingExecutor implements personalized greeting generation.
// This example demonstrates string manipulation, optional parameters,
// and conditional logic in tool execution.
type GreetingExecutor struct{}

// Execute generates a personalized greeting based on the provided parameters.
func (g *GreetingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	// Extract required name parameter
	name, ok := params["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name parameter must be a string")
	}

	// Validate name is not empty
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("name cannot be empty")
	}

	// Extract optional parameters with defaults
	language := "english"
	if lang, exists := params["language"]; exists {
		if langStr, ok := lang.(string); ok {
			language = strings.ToLower(strings.TrimSpace(langStr))
		}
	}

	style := "casual"
	if styleParam, exists := params["style"]; exists {
		if styleStr, ok := styleParam.(string); ok {
			style = strings.ToLower(strings.TrimSpace(styleStr))
		}
	}

	// Extract time-based greeting option
	timeOfDay := ""
	if timeParam, exists := params["time_of_day"]; exists {
		if timeStr, ok := timeParam.(string); ok {
			timeOfDay = strings.ToLower(strings.TrimSpace(timeStr))
		}
	}

	// Extract optional title
	title := ""
	if titleParam, exists := params["title"]; exists {
		if titleStr, ok := titleParam.(string); ok {
			title = strings.TrimSpace(titleStr)
		}
	}

	// Generate greeting
	greeting, err := generateGreeting(name, language, style, timeOfDay, title)
	if err != nil {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   err.Error(),
		}, nil
	}

	// Prepare additional data
	response := map[string]interface{}{
		"greeting":     greeting,
		"formatted_name": formatName(name, title),
		"language":     language,
		"style":        style,
		"character_count": len(greeting),
		"word_count":   len(strings.Fields(greeting)),
	}

	if timeOfDay != "" {
		response["time_context"] = timeOfDay
	}

	return &tools.ToolExecutionResult{
		Success: true,
		Data:    response,
		Metadata: map[string]interface{}{
			"generation_time": time.Now().Format(time.RFC3339),
			"personalization": map[string]interface{}{
				"has_title":     title != "",
				"has_time_context": timeOfDay != "",
				"language_detected": language,
				"style_applied":    style,
			},
		},
	}, nil
}

// generateGreeting creates the actual greeting message
func generateGreeting(name, language, style, timeOfDay, title string) (string, error) {
	formattedName := formatName(name, title)
	
	var baseGreeting string
	
	// Language-specific greetings
	switch language {
	case "english":
		baseGreeting = getEnglishGreeting(style, timeOfDay)
	case "spanish":
		baseGreeting = getSpanishGreeting(style, timeOfDay)
	case "french":
		baseGreeting = getFrenchGreeting(style, timeOfDay)
	case "german":
		baseGreeting = getGermanGreeting(style, timeOfDay)
	default:
		return "", fmt.Errorf("unsupported language: %s", language)
	}
	
	return fmt.Sprintf("%s, %s!", baseGreeting, formattedName), nil
}

// formatName applies title formatting if provided
func formatName(name, title string) string {
	if title != "" {
		return fmt.Sprintf("%s %s", title, name)
	}
	return name
}

// Language-specific greeting functions
func getEnglishGreeting(style, timeOfDay string) string {
	timeGreeting := getTimeBasedGreeting(timeOfDay, map[string]string{
		"morning":   "Good morning",
		"afternoon": "Good afternoon", 
		"evening":   "Good evening",
		"night":     "Good night",
	})
	
	if timeGreeting != "" {
		return timeGreeting
	}
	
	switch style {
	case "formal":
		return "Good day"
	case "casual":
		return "Hello"
	case "friendly":
		return "Hey there"
	case "professional":
		return "Greetings"
	default:
		return "Hello"
	}
}

func getSpanishGreeting(style, timeOfDay string) string {
	timeGreeting := getTimeBasedGreeting(timeOfDay, map[string]string{
		"morning":   "Buenos días",
		"afternoon": "Buenas tardes",
		"evening":   "Buenas noches",
		"night":     "Buenas noches",
	})
	
	if timeGreeting != "" {
		return timeGreeting
	}
	
	switch style {
	case "formal":
		return "Saludos"
	case "casual":
		return "Hola"
	case "friendly":
		return "¡Hola"
	case "professional":
		return "Estimado/a"
	default:
		return "Hola"
	}
}

func getFrenchGreeting(style, timeOfDay string) string {
	timeGreeting := getTimeBasedGreeting(timeOfDay, map[string]string{
		"morning":   "Bonjour",
		"afternoon": "Bonjour",
		"evening":   "Bonsoir",
		"night":     "Bonsoir",
	})
	
	if timeGreeting != "" {
		return timeGreeting
	}
	
	switch style {
	case "formal":
		return "Bonjour"
	case "casual":
		return "Salut"
	case "friendly":
		return "Coucou"
	case "professional":
		return "Madame/Monsieur"
	default:
		return "Bonjour"
	}
}

func getGermanGreeting(style, timeOfDay string) string {
	timeGreeting := getTimeBasedGreeting(timeOfDay, map[string]string{
		"morning":   "Guten Morgen",
		"afternoon": "Guten Tag",
		"evening":   "Guten Abend",
		"night":     "Gute Nacht",
	})
	
	if timeGreeting != "" {
		return timeGreeting
	}
	
	switch style {
	case "formal":
		return "Guten Tag"
	case "casual":
		return "Hallo"
	case "friendly":
		return "Hi"
	case "professional":
		return "Sehr geehrte/r"
	default:
		return "Hallo"
	}
}

// getTimeBasedGreeting returns time-specific greeting if available
func getTimeBasedGreeting(timeOfDay string, greetings map[string]string) string {
	if greeting, exists := greetings[timeOfDay]; exists {
		return greeting
	}
	return ""
}

// CreateGreetingTool creates and configures the greeting tool.
func CreateGreetingTool() *tools.Tool {
	return &tools.Tool{
		ID:          "personalized_greeting",
		Name:        "Personalized Greeting Generator",
		Description: "Generates personalized greetings in multiple languages and styles with optional time-based context",
		Version:     "1.2.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"name": {
					Type:        "string",
					Description: "The name of the person to greet",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{100}[0],
					Pattern:     `^[a-zA-Z\s\-'\.]+$`,
				},
				"language": {
					Type:        "string",
					Description: "The language for the greeting",
					Enum: []interface{}{
						"english", "spanish", "french", "german",
					},
					Default: "english",
				},
				"style": {
					Type:        "string",
					Description: "The style of greeting",
					Enum: []interface{}{
						"casual", "formal", "friendly", "professional",
					},
					Default: "casual",
				},
				"time_of_day": {
					Type:        "string",
					Description: "Time context for the greeting (optional)",
					Enum: []interface{}{
						"morning", "afternoon", "evening", "night",
					},
				},
				"title": {
					Type:        "string",
					Description: "Optional title or honorific (e.g., Dr., Mr., Ms.)",
					MaxLength:   &[]int{20}[0],
					Pattern:     `^[a-zA-Z\.]+$`,
				},
			},
			Required: []string{"name"},
		},
		Metadata: &tools.ToolMetadata{
			Author:        "AG-UI SDK Examples",
			License:       "MIT",
			Documentation: "https://github.com/mattsp1290/ag-ui/blob/main/go-sdk/examples/tools/basic/README.md",
			Tags:          []string{"greeting", "localization", "personalization", "text"},
			Examples: []tools.ToolExample{
				{
					Name:        "Simple Greeting",
					Description: "Basic greeting with default settings",
					Input: map[string]interface{}{
						"name": "Alice",
					},
					Output: map[string]interface{}{
						"greeting": "Hello, Alice!",
						"language": "english",
						"style":    "casual",
					},
				},
				{
					Name:        "Formal Spanish Greeting",
					Description: "Formal greeting in Spanish with title",
					Input: map[string]interface{}{
						"name":     "García",
						"language": "spanish",
						"style":    "formal",
						"title":    "Dr.",
					},
					Output: map[string]interface{}{
						"greeting": "Saludos, Dr. García!",
						"language": "spanish",
						"style":    "formal",
					},
				},
				{
					Name:        "Time-based Greeting",
					Description: "Morning greeting in English",
					Input: map[string]interface{}{
						"name":        "Bob",
						"time_of_day": "morning",
					},
					Output: map[string]interface{}{
						"greeting": "Good morning, Bob!",
						"time_context": "morning",
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
			Timeout:    5 * time.Second,
		},
		Executor: &GreetingExecutor{},
	}
}

func main() {
	// Create registry and register the greeting tool
	registry := tools.NewRegistry()
	greetingTool := CreateGreetingTool()

	if err := registry.Register(greetingTool); err != nil {
		log.Fatalf("Failed to register greeting tool: %v", err)
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
		{"name": "Alice"},
		{"name": "Dr. Smith", "style": "formal", "language": "english"},
		{"name": "María", "language": "spanish", "style": "friendly"},
		{"name": "Jean", "language": "french", "time_of_day": "morning"},
		{"name": "Schmidt", "language": "german", "style": "professional", "title": "Herr"},
	}

	fmt.Println("=== Personalized Greeting Tool Example ===")
	fmt.Println("Demonstrates: String manipulation, optional parameters, conditional logic, and localization")
	fmt.Println()

	for i, params := range examples {
		fmt.Printf("Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "personalized_greeting", params)
		if err != nil {
			fmt.Printf("  Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("  Failed: %s\n", result.Error)
		} else {
			fmt.Printf("  Greeting: %s\n", result.Data.(map[string]interface{})["greeting"])
			if result.Metadata != nil {
				if personalization := result.Metadata["personalization"]; personalization != nil {
					fmt.Printf("  Personalization: %v\n", personalization)
				}
			}
			fmt.Printf("  Duration: %v\n", result.Duration)
		}
		fmt.Println()
	}

	// Demonstrate parameter validation
	fmt.Println("=== Parameter Validation Examples ===")
	
	validationExamples := []map[string]interface{}{
		{"name": ""},                               // Empty name
		{"name": "Alice123"},                       // Invalid characters
		{"name": "Alice", "language": "invalid"},   // Invalid language
		{"name": "Alice", "style": "unknown"},      // Invalid style
	}

	for i, params := range validationExamples {
		fmt.Printf("Validation Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "personalized_greeting", params)
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