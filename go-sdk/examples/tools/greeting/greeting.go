package greeting

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/tools"
)

// GreetingExecutor provides a personalized greeting generator with multi-language support.
// This example demonstrates string manipulation, internationalization, parameter validation,
// and conditional logic based on user preferences.
type GreetingExecutor struct{}

// Execute generates personalized greetings based on the provided parameters.
func (g *GreetingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "name parameter is required",
		}, nil
	}

	// Get optional parameters with defaults
	language := "english"
	if lang, ok := params["language"].(string); ok && lang != "" {
		language = lang
	}

	style := "formal"
	if s, ok := params["style"].(string); ok && s != "" {
		style = s
	}

	timeOfDay := ""
	if tod, ok := params["time_of_day"].(string); ok {
		timeOfDay = tod
	}

	includeEmoji := false
	if emoji, ok := params["include_emoji"].(bool); ok {
		includeEmoji = emoji
	}

	personalize := false
	if p, ok := params["personalize"].(bool); ok {
		personalize = p
	}

	// Generate greeting based on parameters
	greeting := generateGreeting(name, language, style, timeOfDay, includeEmoji, personalize)

	return &tools.ToolExecutionResult{
		Success:   true,
		Data:      greeting,
		Timestamp: time.Now(),
		Duration:  time.Microsecond * 50,
		Metadata: map[string]interface{}{
			"name":          name,
			"language":      language,
			"style":         style,
			"time_of_day":   timeOfDay,
			"include_emoji": includeEmoji,
			"personalize":   personalize,
			"greeting_length": len(greeting),
		},
	}, nil
}

func generateGreeting(name, language, style, timeOfDay string, includeEmoji, personalize bool) string {
	var greeting string

	// Base greetings by language
	greetings := map[string]map[string]string{
		"english": {
			"formal":   "Hello",
			"casual":   "Hi",
			"friendly": "Hey there",
		},
		"spanish": {
			"formal":   "Buenos días",
			"casual":   "Hola",
			"friendly": "¡Hola",
		},
		"french": {
			"formal":   "Bonjour",
			"casual":   "Salut",
			"friendly": "Coucou",
		},
		"german": {
			"formal":   "Guten Tag",
			"casual":   "Hallo",
			"friendly": "Hi",
		},
		"japanese": {
			"formal":   "こんにちは",
			"casual":   "やあ",
			"friendly": "こんにちは",
		},
	}

	// Get base greeting
	if langGreetings, ok := greetings[language]; ok {
		if baseGreeting, ok := langGreetings[style]; ok {
			greeting = baseGreeting
		} else {
			greeting = langGreetings["formal"]
		}
	} else {
		greeting = "Hello"
	}

	// Add time-specific greeting
	if timeOfDay != "" {
		timeGreetings := map[string]map[string]string{
			"english": {
				"morning":   "Good morning",
				"afternoon": "Good afternoon",
				"evening":   "Good evening",
				"night":     "Good night",
			},
			"spanish": {
				"morning":   "Buenos días",
				"afternoon": "Buenas tardes",
				"evening":   "Buenas tardes",
				"night":     "Buenas noches",
			},
			"french": {
				"morning":   "Bonjour",
				"afternoon": "Bonjour",
				"evening":   "Bonsoir",
				"night":     "Bonne nuit",
			},
		}

		if timeGreets, ok := timeGreetings[language]; ok {
			if timeGreet, ok := timeGreets[timeOfDay]; ok {
				greeting = timeGreet
			}
		}
	}

	// Add name
	if style == "formal" {
		greeting += ", " + name
	} else {
		greeting += " " + name
	}

	// Add emoji if requested
	if includeEmoji {
		emojiMap := map[string]string{
			"formal":   "😊",
			"casual":   "👋",
			"friendly": "🤗",
		}
		if emoji, ok := emojiMap[style]; ok {
			greeting += " " + emoji
		}
	}

	// Add personalization
	if personalize {
		personalizations := []string{
			"Hope you're having a great day!",
			"It's wonderful to see you!",
			"Thanks for being awesome!",
		}
		// Use name length to deterministically select personalization
		idx := len(name) % len(personalizations)
		greeting += " " + personalizations[idx]
	}

	// Add punctuation
	if style == "formal" {
		greeting += "."
	} else {
		greeting += "!"
	}

	return greeting
}

// CreateGreetingTool creates and configures the greeting tool.
func CreateGreetingTool() *tools.Tool {
	return &tools.Tool{
		ID:          "greeting",
		Name:        "Greeting",
		Description: "A personalized greeting generator with multi-language support",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"name": {
					Type:        "string",
					Description: "Name of the person to greet",
					MinLength:   func() *int { v := 1; return &v }(),
					MaxLength:   func() *int { v := 100; return &v }(),
				},
				"language": {
					Type:        "string",
					Description: "Language for the greeting",
					Enum:        []interface{}{"english", "spanish", "french", "german", "japanese"},
					Default:     "english",
				},
				"style": {
					Type:        "string",
					Description: "Style of greeting",
					Enum:        []interface{}{"formal", "casual", "friendly"},
					Default:     "formal",
				},
				"time_of_day": {
					Type:        "string",
					Description: "Time of day for contextual greeting",
					Enum:        []interface{}{"morning", "afternoon", "evening", "night"},
				},
				"include_emoji": {
					Type:        "boolean",
					Description: "Include emoji in the greeting",
					Default:     false,
				},
				"personalize": {
					Type:        "boolean",
					Description: "Add personalized message",
					Default:     false,
				},
			},
			Required: []string{"name"},
		},
		Executor: &GreetingExecutor{},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      false,
			Cancelable: false,
			Cacheable:  true,
			Timeout:    5 * time.Second,
		},
		Metadata: &tools.ToolMetadata{
			Author:   "UI Team",
			License:  "MIT",
			Tags:     []string{"greeting", "internationalization", "personalization"},
			Examples: []tools.ToolExample{
				{
					Name:        "Simple Greeting",
					Description: "Basic greeting in English",
					Input: map[string]interface{}{
						"name": "Alice",
					},
					Output: "Hello, Alice.",
				},
				{
					Name:        "Casual Spanish Greeting",
					Description: "Casual greeting in Spanish",
					Input: map[string]interface{}{
						"name":     "Carlos",
						"language": "spanish",
						"style":    "casual",
					},
					Output: "Hola Carlos!",
				},
			},
		},
	}
}

// RunGreetingExample demonstrates the greeting tool functionality
func RunGreetingExample() error {
	// Create registry and register the greeting tool
	registry := tools.NewRegistry()
	greetingTool := CreateGreetingTool()

	if err := registry.Register(greetingTool); err != nil {
		return fmt.Errorf("failed to register greeting tool: %w", err)
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

	fmt.Println("=== Greeting Tool Example ===")
	fmt.Println("Demonstrates: Multi-language support, personalization, and conditional formatting")
	fmt.Println()

	// Test different greeting styles and languages
	greetingExamples := []map[string]interface{}{
		{"name": "Alice"},
		{"name": "Bob", "style": "casual"},
		{"name": "Carlos", "language": "spanish", "style": "casual"},
		{"name": "Marie", "language": "french", "style": "friendly"},
		{"name": "David", "time_of_day": "morning"},
		{"name": "Emma", "time_of_day": "evening", "include_emoji": true},
		{"name": "Frank", "personalize": true},
		{"name": "Grace", "language": "japanese", "style": "friendly", "include_emoji": true, "personalize": true},
	}

	for i, params := range greetingExamples {
		fmt.Printf("%d. Greeting for %s", i+1, params["name"])
		if len(params) > 1 {
			details := make([]string, 0)
			for key, value := range params {
				if key != "name" {
					details = append(details, fmt.Sprintf("%s=%v", key, value))
				}
			}
			if len(details) > 0 {
				fmt.Printf(" (%s)", strings.Join(details, ", "))
			}
		}
		fmt.Print(": ")

		result, err := engine.Execute(ctx, "greeting", params)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("Failed: %s\n", result.Error)
		} else {
			fmt.Printf("%s\n", result.Data.(string))
		}
	}
	fmt.Println()

	// Test all supported languages
	fmt.Println("=== Multi-Language Support ===")
	languages := []string{"english", "spanish", "french", "german", "japanese"}
	
	for _, language := range languages {
		params := map[string]interface{}{
			"name":     "World",
			"language": language,
			"style":    "casual",
		}
		
		result, err := engine.Execute(ctx, "greeting", params)
		if err != nil {
			fmt.Printf("%s: Error: %v\n", language, err)
		} else if !result.Success {
			fmt.Printf("%s: Failed: %s\n", language, result.Error)
		} else {
			fmt.Printf("%s: %s\n", strings.Title(language), result.Data.(string))
		}
	}
	fmt.Println()

	// Test error conditions
	fmt.Println("=== Error Handling Examples ===")
	
	errorExamples := []map[string]interface{}{
		{}, // Missing name
		{"name": ""}, // Empty name
		{"name": 123}, // Invalid name type
	}

	for i, params := range errorExamples {
		fmt.Printf("Error Example %d: %v\n", i+1, params)
		
		result, err := engine.Execute(ctx, "greeting", params)
		if err != nil {
			fmt.Printf("  Execution Error: %v\n", err)
		} else if !result.Success {
			fmt.Printf("  Validation Error: %s\n", result.Error)
		} else {
			fmt.Printf("  Unexpected Success: %v\n", result.Data)
		}
		fmt.Println()
	}

	return nil
}