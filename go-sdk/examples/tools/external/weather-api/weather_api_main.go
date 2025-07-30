package weatherapi

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

func RunWeatherApiExample() error {
	// Create registry and register the weather API tool
	registry := tools.NewRegistry()
	weatherTool := CreateWeatherAPITool()

	if err := registry.Register(weatherTool); err != nil {
		log.Fatalf("Failed to register weather API tool: %v", err)
	}

	// Create execution engine with caching and rate limiting
	engine := tools.NewExecutionEngine(registry,
		tools.WithCaching(1000, 15*time.Minute),
		tools.WithDefaultTimeout(30*time.Second),
	)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Weather API Integration Tool Example ===")
	fmt.Println("Demonstrates: External API integration, HTTP clients, rate limiting, and caching")
	fmt.Println()

	// Example 1: Current weather
	fmt.Println("1. Getting current weather for London...")
	result, err := engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "current",
		"location":  "London, UK",
		"options": map[string]interface{}{
			"units":  "metric",
			"alerts": true,
		},
	})
	printWeatherResult(result, err, "Current weather")

	// Example 2: Weather forecast
	fmt.Println("2. Getting weather forecast for New York...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "forecast",
		"location":  "New York, NY",
		"options": map[string]interface{}{
			"days":  5,
			"hours": false,
			"units": "imperial",
		},
	})
	printWeatherResult(result, err, "Weather forecast")

	// Example 3: Location search
	fmt.Println("3. Searching for locations...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "search",
		"location":  "Tokyo",
	})
	printWeatherResult(result, err, "Location search")

	// Example 4: Weather alerts
	fmt.Println("4. Getting weather alerts for Miami...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "alerts",
		"location":  "Miami, FL",
		"options": map[string]interface{}{
			"units": "imperial",
		},
	})
	printWeatherResult(result, err, "Weather alerts")

	// Example 5: Historical weather (with error handling)
	fmt.Println("5. Getting historical weather data...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "history",
		"location":  "Moscow, Russia",
		"options": map[string]interface{}{
			"date":  "2024-01-01",
			"units": "metric",
		},
	})
	printWeatherResult(result, err, "Historical weather")

	// Example 6: Test caching (same request as example 1)
	fmt.Println("6. Testing cache with same London request...")
	result, err = engine.Execute(ctx, "weather_api", map[string]interface{}{
		"operation": "current",
		"location":  "London, UK",
		"options": map[string]interface{}{
			"units":  "metric",
			"alerts": true,
		},
	})
	printWeatherResult(result, err, "Cached weather")
	
	return nil
}

func printWeatherResult(result *tools.ToolExecutionResult, err error, title string) {
	fmt.Printf("=== %s ===\n", title)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		return
	}

	if !result.Success {
		fmt.Printf("  Failed: %s\n", result.Error)
		if metadata := result.Metadata; metadata != nil {
			if rateLimited, exists := metadata["rate_limited"]; exists && rateLimited.(bool) {
				fmt.Printf("  Rate limited - retry after: %v seconds\n", metadata["retry_after"])
			}
		}
		fmt.Println()
		return
	}

	data := result.Data.(map[string]interface{})
	
	// Handle different operation types
	if _, exists := data["weather_data"]; exists {
		// Weather data response
		fmt.Printf("  Success: Weather data retrieved\n")
		if summary, exists := data["summary"]; exists {
			summaryMap := summary.(map[string]interface{})
			fmt.Printf("  Location: %v, %v\n", summaryMap["location"], summaryMap["country"])
			fmt.Printf("  Temperature: %v°\n", summaryMap["temperature"])
			fmt.Printf("  Condition: %v\n", summaryMap["condition"])
			fmt.Printf("  Last updated: %v\n", summaryMap["last_updated"])
		}
		
		if forecastDays, exists := data["forecast_days"]; exists {
			fmt.Printf("  Forecast days: %v\n", forecastDays)
		}
		
		if alertCount, exists := data["active_alerts"]; exists {
			fmt.Printf("  Active alerts: %v\n", alertCount)
			if alertSummary, exists := data["alert_summary"]; exists {
				summary := alertSummary.(map[string]interface{})
				fmt.Printf("  Most recent alert: %v\n", summary["most_recent_title"])
			}
		}
		
		if cacheHit, exists := data["cache_hit"]; exists && cacheHit.(bool) {
			fmt.Printf("  Data source: Cache (hit)\n")
		} else {
			fmt.Printf("  Data source: API (fresh)\n")
		}
	} else if locations, exists := data["locations"]; exists {
		// Location search response
		fmt.Printf("  Success: Found %v location(s)\n", data["count"])
		locationList := locations.([]interface{})
		for i, loc := range locationList {
			if i < 3 { // Show first 3 results
				locMap := loc.(map[string]interface{})
				fmt.Printf("    %v: %v, %v (%.4f, %.4f)\n", 
					i+1, locMap["name"], locMap["country"], 
					locMap["latitude"], locMap["longitude"])
			}
		}
		if len(locationList) > 3 {
			fmt.Printf("    ... and %d more results\n", len(locationList)-3)
		}
	}

	// Show metadata
	if metadata := result.Metadata; metadata != nil {
		fmt.Printf("  Response time: %vms\n", metadata["response_time_ms"])
		if rateLimitRemaining, exists := metadata["rate_limit_remaining"]; exists {
			fmt.Printf("  Rate limit remaining: %v requests\n", rateLimitRemaining)
		}
		if cached, exists := metadata["cached"]; exists && cached.(bool) {
			fmt.Printf("  Served from cache\n")
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}

// WeatherAPIExecutor provides a simple weather API implementation
type WeatherAPIExecutor struct{}

// Execute simulates weather API calls for demonstration purposes
func (w *WeatherAPIExecutor) Execute(ctx context.Context, params map[string]interface{}) (*tools.ToolExecutionResult, error) {
	operation, ok := params["operation"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "operation parameter is required",
		}, nil
	}

	location, ok := params["location"].(string)
	if !ok {
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   "location parameter is required",
		}, nil
	}

	// Simulate different operations
	switch operation {
	case "current":
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"weather_data": map[string]interface{}{
					"temperature": 22.5,
					"humidity":    65,
					"pressure":    1013.25,
				},
				"summary": map[string]interface{}{
					"location":     location,
					"country":      "United Kingdom",
					"temperature":  "22.5°C",
					"condition":    "Partly cloudy",
					"last_updated": "2024-01-15 14:30:00",
				},
				"active_alerts": 0,
			},
			Metadata: map[string]interface{}{
				"response_time_ms":     120,
				"rate_limit_remaining": 950,
				"cached":               false,
			},
		}, nil
	case "forecast":
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"weather_data": map[string]interface{}{
					"forecast": []interface{}{
						map[string]interface{}{"day": 1, "temp_high": 24, "temp_low": 18},
						map[string]interface{}{"day": 2, "temp_high": 26, "temp_low": 20},
					},
				},
				"summary": map[string]interface{}{
					"location":    location,
					"country":     "United States",
					"temperature": "23°F",
					"condition":   "Sunny",
				},
				"forecast_days": 5,
			},
			Metadata: map[string]interface{}{
				"response_time_ms":     95,
				"rate_limit_remaining": 949,
			},
		}, nil
	case "search":
		return &tools.ToolExecutionResult{
			Success: true,
			Data: map[string]interface{}{
				"locations": []interface{}{
					map[string]interface{}{
						"name":      "Tokyo",
						"country":   "Japan",
						"latitude":  35.6762,
						"longitude": 139.6503,
					},
					map[string]interface{}{
						"name":      "Tokyo Bay",
						"country":   "Japan",
						"latitude":  35.6586,
						"longitude": 139.7454,
					},
				},
				"count": 2,
			},
			Metadata: map[string]interface{}{
				"response_time_ms": 45,
			},
		}, nil
	default:
		return &tools.ToolExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("unsupported operation: %s", operation),
		}, nil
	}
}

// CreateWeatherAPITool creates and configures the weather API tool
func CreateWeatherAPITool() *tools.Tool {
	return &tools.Tool{
		ID:          "weather_api",
		Name:        "Weather API Integration",
		Description: "Demonstrates external API integration with weather services",
		Version:     "1.0.0",
		Schema: &tools.ToolSchema{
			Type: "object",
			Properties: map[string]*tools.Property{
				"operation": {
					Type:        "string",
					Description: "Weather operation to perform",
					Enum: []interface{}{
						"current", "forecast", "search", "alerts", "history",
					},
				},
				"location": {
					Type:        "string",
					Description: "Location for weather data",
					MinLength:   &[]int{1}[0],
					MaxLength:   &[]int{100}[0],
				},
				"options": {
					Type:        "object",
					Description: "Additional options",
					Properties: map[string]*tools.Property{
						"units": {
							Type: "string",
							Enum: []interface{}{"metric", "imperial"},
						},
						"days": {
							Type:    "number",
							Minimum: &[]float64{1}[0],
							Maximum: &[]float64{10}[0],
						},
					},
				},
			},
			Required: []string{"operation", "location"},
		},
		Metadata: &tools.ToolMetadata{
			Author:  "AG-UI SDK Examples",
			License: "MIT",
			Tags:    []string{"weather", "api", "external"},
		},
		Capabilities: &tools.ToolCapabilities{
			Streaming:  false,
			Async:      true,
			Cancelable: true,
			Timeout:    30 * time.Second,
		},
		Executor: &WeatherAPIExecutor{},
	}
}