package greeting_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/ag-ui/go-sdk/pkg/tools"
)

// MockGreetingExecutor provides a testable version of the greeting tool
type MockGreetingExecutor struct{}

func (g *MockGreetingExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	name, ok := params["name"].(string)
	if !ok || name == "" {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "name parameter is required",
		}, nil
	}

	// Get optional parameters
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

func createGreetingTool() *tools.Tool {
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
		Executor: &MockGreetingExecutor{},
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

// TestGreetingTool_BasicGreetings tests basic greeting functionality
func TestGreetingTool_BasicGreetings(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	testCases := []struct {
		name            string
		params          map[string]interface{}
		expectedContains []string
		shouldErr       bool
	}{
		{
			name: "Simple English greeting",
			params: map[string]interface{}{
				"name": "Alice",
			},
			expectedContains: []string{"Hello", "Alice"},
		},
		{
			name: "Casual English greeting",
			params: map[string]interface{}{
				"name":  "Bob",
				"style": "casual",
			},
			expectedContains: []string{"Hi", "Bob", "!"},
		},
		{
			name: "Formal Spanish greeting",
			params: map[string]interface{}{
				"name":     "Carlos",
				"language": "spanish",
				"style":    "formal",
			},
			expectedContains: []string{"Buenos días", "Carlos"},
		},
		{
			name: "French friendly greeting",
			params: map[string]interface{}{
				"name":     "Marie",
				"language": "french",
				"style":    "friendly",
			},
			expectedContains: []string{"Coucou", "Marie"},
		},
		{
			name: "Morning greeting",
			params: map[string]interface{}{
				"name":        "David",
				"time_of_day": "morning",
			},
			expectedContains: []string{"Good morning", "David"},
		},
		{
			name: "Evening greeting with emoji",
			params: map[string]interface{}{
				"name":          "Emma",
				"time_of_day":   "evening",
				"include_emoji": true,
			},
			expectedContains: []string{"Good evening", "Emma", "😊"},
		},
		{
			name: "Personalized greeting",
			params: map[string]interface{}{
				"name":        "Frank",
				"personalize": true,
			},
			expectedContains: []string{"Hello", "Frank", "Thanks for being awesome"},
>		},
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
			
			greeting, ok := result.Data.(string)
			require.True(t, ok, "Result should be a string")
			assert.NotEmpty(t, greeting)
			
			// Check that all expected strings are present
			for _, expected := range tc.expectedContains {
				assert.Contains(t, greeting, expected, 
					"Greeting '%s' should contain '%s'", greeting, expected)
			}
			
			// Check metadata
			assert.NotNil(t, result.Metadata)
			assert.Equal(t, tc.params["name"], result.Metadata["name"])
		})
	}
}

// TestGreetingTool_AllLanguages tests all supported languages
func TestGreetingTool_AllLanguages(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	languages := []string{"english", "spanish", "french", "german", "japanese"}
	
	for _, language := range languages {
		t.Run("Language_"+language, func(t *testing.T) {
			params := map[string]interface{}{
				"name":     "Test",
				"language": language,
			}
			
			result, err := tool.Executor.Execute(ctx, params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			
			greeting, ok := result.Data.(string)
			require.True(t, ok)
			assert.NotEmpty(t, greeting)
			assert.Contains(t, greeting, "Test")
		})
	}
}

// TestGreetingTool_AllStyles tests all greeting styles
func TestGreetingTool_AllStyles(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	styles := []string{"formal", "casual", "friendly"}
	
	for _, style := range styles {
		t.Run("Style_"+style, func(t *testing.T) {
			params := map[string]interface{}{
				"name":  "Test",
				"style": style,
			}
			
			result, err := tool.Executor.Execute(ctx, params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			
			greeting, ok := result.Data.(string)
			require.True(t, ok)
			assert.NotEmpty(t, greeting)
			
			// Formal style should end with period, others with exclamation
			if style == "formal" {
				assert.True(t, strings.HasSuffix(greeting, "."), 
					"Formal greeting should end with period: %s", greeting)
			} else {
				assert.True(t, strings.HasSuffix(greeting, "!"), 
					"Non-formal greeting should end with exclamation: %s", greeting)
			}
		})
	}
}

// TestGreetingTool_TimeOfDay tests time-specific greetings
func TestGreetingTool_TimeOfDay(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	timeOfDayTests := map[string]string{
		"morning":   "Good morning",
		"afternoon": "Good afternoon",
		"evening":   "Good evening",
		"night":     "Good night",
	}

	for timeOfDay, expectedGreeting := range timeOfDayTests {
		t.Run("TimeOfDay_"+timeOfDay, func(t *testing.T) {
			params := map[string]interface{}{
				"name":        "Test",
				"time_of_day": timeOfDay,
			}
			
			result, err := tool.Executor.Execute(ctx, params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			
			greeting, ok := result.Data.(string)
			require.True(t, ok)
			assert.Contains(t, greeting, expectedGreeting)
		})
	}
}

// TestGreetingTool_ErrorHandling tests error conditions
func TestGreetingTool_ErrorHandling(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	testCases := []struct {
		name          string
		params        map[string]interface{}
		expectedError string
	}{
		{
			name:          "Missing name",
			params:        map[string]interface{}{},
			expectedError: "name parameter is required",
		},
		{
			name: "Empty name",
			params: map[string]interface{}{
				"name": "",
			},
			expectedError: "name parameter is required",
		},
		{
			name: "Invalid name type",
			params: map[string]interface{}{
				"name": 123,
			},
			expectedError: "name parameter is required",
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

// TestGreetingTool_InvalidParameterValues tests invalid parameter values
func TestGreetingTool_InvalidParameterValues(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	// Test with invalid language (should fallback to default behavior)
	params := map[string]interface{}{
		"name":     "Test",
		"language": "klingon", // Not supported
	}
	
	result, err := tool.Executor.Execute(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success) // Should still work with fallback
	
	greeting, ok := result.Data.(string)
	require.True(t, ok)
	assert.Contains(t, greeting, "Test")
}

// TestGreetingTool_SpecialCharacters tests names with special characters
func TestGreetingTool_SpecialCharacters(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	specialNames := []string{
		"José",
		"François",
		"김민수",
		"山田太郎",
		"O'Connor",
		"Van Der Berg",
		"Jean-Pierre",
		"Mary-Jane",
	}

	for _, name := range specialNames {
		t.Run("SpecialName_"+name, func(t *testing.T) {
			params := map[string]interface{}{
				"name": name,
			}
			
			result, err := tool.Executor.Execute(ctx, params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			
			greeting, ok := result.Data.(string)
			require.True(t, ok)
			assert.Contains(t, greeting, name)
		})
	}
}

// TestGreetingTool_EmojiCombinations tests emoji functionality
func TestGreetingTool_EmojiCombinations(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	styles := []string{"formal", "casual", "friendly"}
	
	for _, style := range styles {
		t.Run("Emoji_"+style, func(t *testing.T) {
			params := map[string]interface{}{
				"name":          "Test",
				"style":         style,
				"include_emoji": true,
			}
			
			result, err := tool.Executor.Execute(ctx, params)
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.True(t, result.Success)
			
			greeting, ok := result.Data.(string)
			require.True(t, ok)
			
			// Should contain some emoji
			emojiPresent := strings.Contains(greeting, "😊") ||
						   strings.Contains(greeting, "👋") ||
						   strings.Contains(greeting, "🤗")
			assert.True(t, emojiPresent, "Greeting should contain emoji: %s", greeting)
		})
	}
}

// TestGreetingTool_Personalization tests personalization feature
func TestGreetingTool_Personalization(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	params := map[string]interface{}{
		"name":        "Test",
		"personalize": true,
	}
	
	result, err := tool.Executor.Execute(ctx, params)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	
	greeting, ok := result.Data.(string)
	require.True(t, ok)
	
	// Should contain personalized message
	personalizedPhrases := []string{
		"Hope you're having",
		"wonderful to see",
		"Thanks for being",
	}
	
	hasPersonalization := false
	for _, phrase := range personalizedPhrases {
		if strings.Contains(greeting, phrase) {
			hasPersonalization = true
			break
		}
	}
	
	assert.True(t, hasPersonalization, "Greeting should contain personalization: %s", greeting)
}

// TestGreetingTool_ConsistentOutput tests that same inputs produce same outputs
func TestGreetingTool_ConsistentOutput(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	params := map[string]interface{}{
		"name":          "Test",
		"language":      "english",
		"style":         "casual",
		"include_emoji": true,
		"personalize":   true,
	}

	// Run the same greeting multiple times
	var greetings []string
	for i := 0; i < 5; i++ {
		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, result.Success)
		
		greeting, ok := result.Data.(string)
		require.True(t, ok)
		greetings = append(greetings, greeting)
	}

	// All greetings should be identical
	for i := 1; i < len(greetings); i++ {
		assert.Equal(t, greetings[0], greetings[i], 
			"Greetings should be consistent across runs")
	}
}

// TestGreetingTool_Performance tests performance characteristics
func TestGreetingTool_Performance(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	params := map[string]interface{}{
		"name":          "PerformanceTest",
		"language":      "english",
		"style":         "friendly",
		"include_emoji": true,
		"personalize":   true,
	}

	// Test single greeting performance
	start := time.Now()
	result, err := tool.Executor.Execute(ctx, params)
	duration := time.Since(start)

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.Success)
	
	// Should complete very quickly
	assert.Less(t, duration, time.Millisecond, "Greeting should complete within 1ms")

	// Test bulk greeting performance
	numGreetings := 1000
	start = time.Now()
	
	for i := 0; i < numGreetings; i++ {
		result, err := tool.Executor.Execute(ctx, params)
		require.NoError(t, err)
		require.True(t, result.Success)
	}
	
	totalDuration := time.Since(start)
	avgDuration := totalDuration / time.Duration(numGreetings)
	
	assert.Less(t, avgDuration, time.Millisecond, "Average greeting should complete within 1ms")
	
	t.Logf("Generated %d greetings in %v (avg: %v per greeting)", 
		numGreetings, totalDuration, avgDuration)
}

// TestGreetingTool_Concurrency tests concurrent execution
func TestGreetingTool_Concurrency(t *testing.T) {
	tool := createGreetingTool()
	ctx := context.Background()

	const numGoroutines = 10
	const greetingsPerGoroutine = 100

	results := make(chan *tools.ToolExecutionResult, numGoroutines*greetingsPerGoroutine)
	errors := make(chan error, numGoroutines*greetingsPerGoroutine)

	// Start multiple goroutines generating greetings
	for i := 0; i < numGoroutines; i++ {
		go func(goroutineID int) {
			for j := 0; j < greetingsPerGoroutine; j++ {
				params := map[string]interface{}{
					"name":     fmt.Sprintf("User%d_%d", goroutineID, j),
					"language": []string{"english", "spanish", "french"}[j%3],
					"style":    []string{"formal", "casual", "friendly"}[j%3],
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
	for i := 0; i < numGoroutines*greetingsPerGoroutine; i++ {
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

	assert.Equal(t, numGoroutines*greetingsPerGoroutine, successCount)
	assert.Equal(t, 0, errorCount)
}

// TestGreetingTool_Schema tests schema validation
func TestGreetingTool_Schema(t *testing.T) {
	tool := createGreetingTool()

	// Test schema structure
	assert.NotNil(t, tool.Schema)
	assert.Equal(t, "object", tool.Schema.Type)
	assert.Contains(t, tool.Schema.Properties, "name")
	assert.Contains(t, tool.Schema.Properties, "language")
	assert.Contains(t, tool.Schema.Properties, "style")
	assert.Contains(t, tool.Schema.Required, "name")

	// Test enum values
	languageProp := tool.Schema.Properties["language"]
	assert.NotNil(t, languageProp.Enum)
	assert.Contains(t, languageProp.Enum, "english")
	assert.Contains(t, languageProp.Enum, "spanish")
	assert.Contains(t, languageProp.Enum, "french")

	styleProp := tool.Schema.Properties["style"]
	assert.NotNil(t, styleProp.Enum)
	assert.Contains(t, styleProp.Enum, "formal")
	assert.Contains(t, styleProp.Enum, "casual")
	assert.Contains(t, styleProp.Enum, "friendly")
}

// TestGreetingTool_Metadata tests tool metadata
func TestGreetingTool_Metadata(t *testing.T) {
	tool := createGreetingTool()

	assert.Equal(t, "greeting", tool.ID)
	assert.Equal(t, "Greeting", tool.Name)
	assert.NotEmpty(t, tool.Description)
	assert.Equal(t, "1.0.0", tool.Version)

	assert.NotNil(t, tool.Metadata)
	assert.Equal(t, "UI Team", tool.Metadata.Author)
	assert.Equal(t, "MIT", tool.Metadata.License)
	assert.Contains(t, tool.Metadata.Tags, "greeting")
	assert.Contains(t, tool.Metadata.Tags, "internationalization")

	// Test examples
	assert.NotEmpty(t, tool.Metadata.Examples)
	assert.Len(t, tool.Metadata.Examples, 2)
}

// BenchmarkGreetingTool_Languages benchmarks different languages
func BenchmarkGreetingTool_Languages(b *testing.B) {
	tool := createGreetingTool()
	ctx := context.Background()

	languages := []string{"english", "spanish", "french", "german", "japanese"}

	for _, language := range languages {
		b.Run(language, func(b *testing.B) {
			params := map[string]interface{}{
				"name":     "BenchmarkUser",
				"language": language,
				"style":    "casual",
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				result, err := tool.Executor.Execute(ctx, params)
				if err != nil || !result.Success {
					b.Fatalf("Greeting failed: %v", err)
				}
			}
		})
	}
}

// Example test showing how to use the greeting tool
func Example_greetingBasicUsage() {
>	tool := createGreetingTool()
	ctx := context.Background()

	// Simple greeting
	params := map[string]interface{}{
		"name": "Alice",
	}

	result, err := tool.Executor.Execute(ctx, params)
	if err != nil {
		panic(err)
	}

	if result.Success {
		fmt.Println("Greeting:", result.Data.(string))
	} else {
		fmt.Println("Error:", result.Error)
	}

	// Output: Greeting: Hello, Alice.
}

func Example_greetingMultiLanguage() {
>	tool := createGreetingTool()
	ctx := context.Background()

	// Spanish casual greeting with emoji
	params := map[string]interface{}{
		"name":          "Carlos",
		"language":      "spanish",
		"style":         "casual",
		"include_emoji": true,
	}

	result, err := tool.Executor.Execute(ctx, params)
	if err != nil {
		panic(err)
	}

	if result.Success {
		fmt.Println("Greeting:", result.Data.(string))
	} else {
		fmt.Println("Error:", result.Error)
	}

	// Output: Greeting: Hola Carlos 👋!
}