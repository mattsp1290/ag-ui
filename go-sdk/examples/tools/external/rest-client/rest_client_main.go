package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/ag-ui/go-sdk/pkg/tools"
)

func main() {
	// Create registry and register the REST client tool
	registry := tools.NewRegistry()
	restClientTool := CreateRESTClientTool()

	if err := registry.Register(restClientTool); err != nil {
		log.Fatalf("Failed to register REST client tool: %v", err)
	}

	// Create execution engine
	engine := tools.NewExecutionEngine(registry,
		tools.WithDefaultTimeout(60*time.Second),
	)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := engine.Shutdown(ctx); err != nil {
			log.Printf("Engine shutdown error: %v", err)
		}
	}()

	ctx := context.Background()

	fmt.Println("=== Advanced REST API Client Tool Example ===")
	fmt.Println("Demonstrates: HTTP client integration, authentication, error handling, and response analysis")
	fmt.Println()

	// Example 1: Simple GET request
	fmt.Println("1. Simple GET request...")
	result, err := engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/get",
		"query_params": map[string]interface{}{
			"param1": "value1",
			"param2": "value2",
		},
		"headers": map[string]interface{}{
			"Accept": "application/json",
		},
	})
	printRESTResult(result, err, "Simple GET request")

	// Example 2: POST request with JSON body
	fmt.Println("2. POST request with JSON body...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "POST",
		"url":    "https://httpbin.org/post",
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
			"Accept":       "application/json",
		},
		"body": map[string]interface{}{
			"name":    "John Doe",
			"email":   "john@example.com",
			"age":     30,
			"active":  true,
		},
	})
	printRESTResult(result, err, "POST with JSON")

	// Example 3: Basic authentication
	// NOTE: In production, use environment variables for credentials
	fmt.Println("3. Request with Basic authentication...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/basic-auth/user/pass",
		"auth": map[string]interface{}{
			"type": "basic",
			"config": map[string]interface{}{
				"username": "user", // Use os.Getenv("API_USERNAME") for production
				"password": "pass", // Use os.Getenv("API_PASSWORD") for production
			},
		},
	})
	printRESTResult(result, err, "Basic authentication")

	// Example 4: Request with custom options
	fmt.Println("4. Request with custom options and retry policy...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/status/500",
		"options": map[string]interface{}{
			"timeout":     10,
			"verify_ssl":  true,
			"compression": true,
			"user_agent":  "Custom-User-Agent/1.0",
			"retry_policy": map[string]interface{}{
				"max_retries":     2,
				"backoff_delay":   500,
				"retry_on_status": []int{500, 502, 503},
			},
		},
	})
	printRESTResult(result, err, "Custom options with retry")

	// Example 5: API Key authentication in header
	// NOTE: In production, use environment variables for API keys
	fmt.Println("5. API Key authentication in header...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/headers",
		"auth": map[string]interface{}{
			"type": "api_key",
			"config": map[string]interface{}{
				"key":      "demo-api-key-12345", // Use os.Getenv("API_KEY") for production
				"location": "header",
				"name":     "X-API-Key",
			},
		},
	})
	printRESTResult(result, err, "API Key authentication")

	// Example 6: Request with disabled SSL verification (for testing)
	fmt.Println("6. Request with custom SSL and redirect settings...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/redirect/3",
		"options": map[string]interface{}{
			"follow_redirects": true,
			"verify_ssl":       true,
			"timeout":          15,
		},
	})
	printRESTResult(result, err, "SSL and redirect settings")

	// Example 7: Error handling demonstration
	fmt.Println("7. Error handling demonstration...")
	result, err = engine.Execute(ctx, "rest_client", map[string]interface{}{
		"method": "GET",
		"url":    "https://httpbin.org/status/404",
	})
	printRESTResult(result, err, "Error handling (404)")
}

func printRESTResult(result *tools.ToolExecutionResult, err error, title string) {
	fmt.Printf("=== %s ===\n", title)
	
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Println()
		return
	}

	if !result.Success {
		fmt.Printf("  Failed: %s\n", result.Error)
		fmt.Println()
		return
	}

	data := result.Data.(map[string]interface{})
	summary := data["summary"].(map[string]interface{})
	analysis := data["analysis"].(map[string]interface{})
	
	fmt.Printf("  Status: %v (%v)\n", summary["status_code"], summary["status"])
	fmt.Printf("  Response time: %vms\n", summary["response_time"])
	fmt.Printf("  Content length: %v bytes\n", summary["content_length"])
	fmt.Printf("  Redirects: %v\n", summary["redirects"])

	// Show analysis
	fmt.Printf("  Performance: %v\n", analysis["performance"])
	fmt.Printf("  Security grade: %v\n", analysis["security_grade"])
	if contentType, exists := analysis["content_type"]; exists && contentType != nil {
		fmt.Printf("  Content type: %v\n", contentType)
	}
	if sslEnabled, exists := analysis["ssl_enabled"]; exists {
		fmt.Printf("  SSL enabled: %v\n", sslEnabled)
	}

	// Show response data (truncated)
	if response, exists := data["response"]; exists {
		responseMap := response.(map[string]interface{})
		if body, exists := responseMap["body"]; exists {
			bodyStr := fmt.Sprintf("%v", body)
			if len(bodyStr) > 200 {
				bodyStr = bodyStr[:200] + "..."
			}
			fmt.Printf("  Response body (truncated): %s\n", bodyStr)
		}
		
		// Show SSL info if available
		if sslInfo, exists := responseMap["ssl_info"]; exists && sslInfo != nil {
			fmt.Printf("  SSL certificate info available\n")
		}
		
		// Show redirect info if available
		if redirects, exists := responseMap["redirects"]; exists {
			redirectsList := redirects.([]interface{})
			if len(redirectsList) > 0 {
				fmt.Printf("  Followed %d redirect(s)\n", len(redirectsList))
			}
		}
	}

	// Show metadata
	if metadata := result.Metadata; metadata != nil {
		if finalURL, exists := metadata["final_url"]; exists {
			fmt.Printf("  Final URL: %v\n", finalURL)
		}
		if responseSize, exists := metadata["response_size"]; exists {
			fmt.Printf("  Response size: %v bytes\n", responseSize)
		}
	}

	fmt.Printf("  Duration: %v\n", result.Duration)
	fmt.Println()
}