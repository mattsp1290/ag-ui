// Package client provides usage examples demonstrating the new configuration
// system while maintaining backward compatibility with existing code.
package client

import (
	"fmt"
	"time"
	"log"
)

// =============================================================================
// Usage Examples - Before and After
// =============================================================================

// ExampleOldWay demonstrates the old way of creating configurations
func ExampleOldWay() {
	// OLD WAY - Complex nested configuration
	agentConfig := &AgentConfig{
		Name:        "my-agent",
		Description: "My custom agent",
		Capabilities: AgentCapabilities{
			Tools:              []string{"basic"},
			Streaming:          true,
			StateSync:          true,
			MessageHistory:     true,
			CustomCapabilities: make(map[string]interface{}),
		},
		EventProcessing: EventProcessingConfig{
			BufferSize:       100,
			BatchSize:        10,
			Timeout:          30 * time.Second,
			EnableValidation: true,
			EnableMetrics:    true,
		},
		State: StateConfig{
			SyncInterval:       5 * time.Second,
			CacheSize:          "100MB",
			EnablePersistence:  true,
			ConflictResolution: "merge_with_priority",
		},
		Tools: ToolsConfig{
			Timeout:          60 * time.Second,
			MaxConcurrent:    10,
			EnableSandboxing: true,
			EnableCaching:    true,
		},
		History: HistoryConfig{
			MaxMessages: 1000,
			Retention:   24 * time.Hour,
		},
		Custom: make(map[string]interface{}),
	}
	
	httpConfig := &HttpConfig{
		ProtocolVersion:     "HTTP/1.1",
		EnableHTTP2:         false,
		ForceHTTP2:          false,
		MaxIdleConns:        10,
		MaxIdleConnsPerHost: 2,
		MaxConnsPerHost:     10,
		IdleConnTimeout:     30 * time.Second,
		KeepAlive:           30 * time.Second,
		DialTimeout:         10 * time.Second,
		RequestTimeout:      30 * time.Second,
		ResponseTimeout:     30 * time.Second,
		TLSHandshakeTimeout: 10 * time.Second,
		InsecureSkipVerify:  false,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		UserAgent:           "my-app/1.0",
		MaxResponseBodySize: 50 * 1024 * 1024,
		EnableCircuitBreaker: false,
	}
	
	securityConfig := &SecurityConfig{
		AuthMethod:       AuthMethodJWT,
		EnableMultiAuth:  false,
		SupportedMethods: []AuthMethod{AuthMethodJWT},
		JWT: JWTConfig{
			SigningMethod:    "HS256",
			SecretKey:        "my-secret-key",
			AccessTokenTTL:   15 * time.Minute,
			RefreshTokenTTL:  24 * time.Hour,
			AutoRefresh:      true,
			RefreshThreshold: 2 * time.Minute,
			LeewayTime:       30 * time.Second,
			CustomClaims:     make(map[string]interface{}),
		},
		// ... many more nested fields
	}
	
	fmt.Printf("Created agent: %s\n", agentConfig.Name)
	fmt.Printf("HTTP timeout: %v\n", httpConfig.RequestTimeout)
	fmt.Printf("Auth method: %s\n", securityConfig.AuthMethod)
}

// ExampleNewWaySimple demonstrates the new simple way using profiles
func ExampleNewWaySimple() {
	// NEW WAY - Simple profile-based configuration
	agentConfig, httpConfig, securityConfig, _, _, err := NewDevelopmentConfig().
		WithAgentName("my-agent").
		WithAgentDescription("My custom agent").
		WithJWTAuth("HS256", "my-secret-key", 15*time.Minute, 24*time.Hour).
		Build()
	
	if err != nil {
		log.Printf("Configuration error: %v", err)
		return
	}
	
	fmt.Printf("Created agent: %s\n", agentConfig.Name)
	fmt.Printf("HTTP timeout: %v\n", httpConfig.RequestTimeout)
	fmt.Printf("Auth method: %s\n", securityConfig.AuthMethod)
}

// ExampleNewWayAdvanced demonstrates advanced configuration building
func ExampleNewWayAdvanced() {
	// NEW WAY - Advanced builder pattern with validation
	builder := NewConfigBuilder().
		WithAgentName("advanced-agent").
		WithAgentDescription("Advanced configured agent").
		WithHTTPTimeouts(5*time.Second, 30*time.Second, 30*time.Second).
		WithHTTPConnectionLimits(50, 10, 25).
		WithHTTP2(true, false).
		WithJWTAuth("RS256", "", 10*time.Minute, 2*time.Hour).
		WithTLS(true, "/path/to/cert.pem", "/path/to/key.pem", "/path/to/ca.pem").
		WithCORS([]string{"https://example.com"}, []string{"GET", "POST"}, []string{"Authorization"}).
		WithRateLimit(100, 20).
		WithRetry(3, 1*time.Second, 10*time.Second).
		WithCircuitBreaker(true, 10, 3, 60*time.Second).
		ForProduction() // Apply production-ready settings
	
	agentConfig, httpConfig, securityConfig, _, resilienceConfig, err := builder.Build()
	if err != nil {
		log.Printf("Configuration validation failed: %v", err)
		return
	}
	
	fmt.Printf("Created agent: %s\n", agentConfig.Name)
	fmt.Printf("HTTP/2 enabled: %v\n", httpConfig.EnableHTTP2)
	fmt.Printf("TLS enabled: %v\n", securityConfig.TLS.Enabled)
	fmt.Printf("Circuit breaker enabled: %v\n", resilienceConfig.CircuitBreaker.Enabled)
}

// ExampleTemplateUsage demonstrates using predefined templates
func ExampleTemplateUsage() {
	// Using predefined templates
	microserviceAgent, _, _, _, _, err := MicroserviceConfig("user-service", "http://user-service:8080").
		WithJWTAuth("HS256", "service-secret", 30*time.Minute, 24*time.Hour).
		Build()
	
	if err != nil {
		log.Printf("Microservice config error: %v", err)
		return
	}
	
	gatewayAgent, _, _, _, _, err := APIGatewayConfig().
		WithAPIKeyAuth("X-API-Key", "Bearer ").
		WithCORS([]string{"*"}, []string{"GET", "POST", "PUT", "DELETE"}, []string{"*"}).
		Build()
	
	if err != nil {
		log.Printf("Gateway config error: %v", err)
		return
	}
	
	fmt.Printf("Microservice agent: %s\n", microserviceAgent.Name)
	fmt.Printf("Gateway agent: %s\n", gatewayAgent.Name)
}

// ExampleQuickHelpers demonstrates using quick helper functions
func ExampleQuickHelpers() {
	// Quick helper functions for common scenarios
	httpConfig := QuickHTTPConfig(30 * time.Second)
	jwtConfig := QuickJWTConfig("my-secret", 15*time.Minute, 24*time.Hour)
	apiKeyConfig := QuickAPIKeyConfig("X-API-Key", "Bearer ")
	corsConfig := QuickCORSConfig([]string{"https://example.com"})
	retryConfig := QuickRetryConfig(3, 1*time.Second)
	
	fmt.Printf("HTTP timeout: %v\n", httpConfig.RequestTimeout)
	fmt.Printf("JWT signing method: %s\n", jwtConfig.SigningMethod)
	fmt.Printf("API key header: %s\n", apiKeyConfig.HeaderName)
	fmt.Printf("CORS enabled: %v\n", corsConfig.Enabled)
	fmt.Printf("Max retry attempts: %d\n", retryConfig.MaxAttempts)
}

// ExampleBackwardCompatibility demonstrates that old code still works
func ExampleBackwardCompatibility() {
	// Old code continues to work - backward compatibility maintained
	agentConfig := &AgentConfig{
		Name:        "legacy-agent",
		Description: "Legacy agent configuration",
		Capabilities: AgentCapabilities{
			Tools:     []string{"legacy"},
			Streaming: false,
		},
		EventProcessing: EventProcessingConfig{
			BufferSize: 50,
			BatchSize:  5,
			Timeout:    10 * time.Second,
		},
	}
	
	// New validation can be applied to old configurations
	if err := agentConfig.Validate(); err != nil {
		log.Printf("Legacy config validation failed: %v", err)
		return
	}
	
	fmt.Printf("Legacy agent validated: %s\n", agentConfig.Name)
}

// =============================================================================
// Real-World Usage Scenarios
// =============================================================================

// ExampleWebAPIServer demonstrates configuration for a web API server
func ExampleWebAPIServer() {
	// Web API server configuration
	config := NewProductionConfig().
		WithAgentName("web-api-server").
		WithAgentDescription("Production web API server").
		WithHTTPTimeouts(10*time.Second, 60*time.Second, 60*time.Second).
		WithHTTPConnectionLimits(100, 20, 50).
		WithHTTP2(true, false).
		WithJWTAuth("RS256", "", 15*time.Minute, 24*time.Hour).
		WithTLS(true, "/etc/ssl/server.crt", "/etc/ssl/server.key", "/etc/ssl/ca.crt").
		WithCORS([]string{"https://app.example.com", "https://admin.example.com"}, 
			[]string{"GET", "POST", "PUT", "DELETE"}, 
			[]string{"Authorization", "Content-Type"}).
		WithRateLimit(1000, 100).
		WithRetry(3, 500*time.Millisecond, 5*time.Second).
		WithCircuitBreaker(true, 20, 5, 60*time.Second)
	
	agentConfig, httpConfig, securityConfig, _, resilienceConfig, err := config.Build()
	if err != nil {
		log.Fatalf("Web API server config error: %v", err)
	}
	
	fmt.Printf("Web API Server Config:\n")
	fmt.Printf("  Agent: %s\n", agentConfig.Name)
	fmt.Printf("  HTTP/2: %v\n", httpConfig.EnableHTTP2)
	fmt.Printf("  TLS: %v\n", securityConfig.TLS.Enabled)
	fmt.Printf("  Rate Limit: %.0f req/s\n", securityConfig.RateLimit.RequestsPerSecond)
	fmt.Printf("  Circuit Breaker: %v\n", resilienceConfig.CircuitBreaker.Enabled)
}

// ExampleMicroserviceMesh demonstrates configuration for service mesh
func ExampleMicroserviceMesh() {
	// Service A configuration
	serviceA := MicroserviceConfig("user-service", "http://user-service:8080").
		WithJWTAuth("HS256", "shared-secret", 30*time.Minute, 24*time.Hour).
		WithRetry(5, 1*time.Second, 30*time.Second).
		WithCircuitBreaker(true, 10, 3, 60*time.Second)
	
	// Service B configuration
	serviceB := MicroserviceConfig("order-service", "http://order-service:8080").
		WithJWTAuth("HS256", "shared-secret", 30*time.Minute, 24*time.Hour).
		WithRetry(3, 2*time.Second, 15*time.Second).
		WithCircuitBreaker(true, 5, 2, 30*time.Second)
	
	// Gateway configuration
	gateway := APIGatewayConfig().
		WithJWTAuth("RS256", "", 15*time.Minute, 2*time.Hour).
		WithCORS([]string{"https://app.example.com"}, 
			[]string{"GET", "POST", "PUT", "DELETE"}, 
			[]string{"Authorization", "Content-Type"}).
		WithRateLimit(5000, 500)
	
	// Build all configurations
	configs := []*ConfigBuilder{serviceA, serviceB, gateway}
	for i, config := range configs {
		agentConfig, _, _, _, _, err := config.Build()
		if err != nil {
			log.Printf("Service %d config error: %v", i, err)
			continue
		}
		fmt.Printf("Service %d: %s\n", i+1, agentConfig.Name)
	}
}

// ExampleDevelopmentSetup demonstrates development environment setup
func ExampleDevelopmentSetup() {
	// Development environment with easy configuration
	devConfig := NewDevelopmentConfig().
		WithAgentName("dev-api").
		WithJWTAuth("HS256", "dev-secret-key", 1*time.Hour, 24*time.Hour). // Longer tokens for dev
		WithInsecureTLS(). // Allow self-signed certificates
		WithCORS([]string{"*"}, []string{"*"}, []string{"*"}) // Permissive CORS
	
	agentConfig, httpConfig, securityConfig, _, _, err := devConfig.Build()
	if err != nil {
		log.Printf("Dev config error: %v", err)
		return
	}
	
	fmt.Printf("Development Setup:\n")
	fmt.Printf("  Agent: %s\n", agentConfig.Name)
	fmt.Printf("  Insecure TLS: %v\n", httpConfig.InsecureSkipVerify)
	fmt.Printf("  CORS Origins: %v\n", securityConfig.SecurityHeaders.CORSConfig.AllowedOrigins)
	fmt.Printf("  JWT TTL: %v\n", securityConfig.JWT.AccessTokenTTL)
}

// ExampleConfigValidation demonstrates configuration validation
func ExampleConfigValidation() {
	// Example of configuration that will fail validation
	badConfig := NewConfigBuilder().
		WithAgentName(""). // Empty name - will fail validation
		WithHTTPTimeouts(-1*time.Second, 0, 0). // Negative timeout - will fail validation
		WithJWTAuth("", "", 0, 0) // Empty signing method and zero TTL - will fail validation
	
	// Validation will catch these errors
	if err := badConfig.Validate(); err != nil {
		fmt.Printf("Configuration validation failed (as expected): %v\n", err)
	}
	
	// Good configuration
	goodConfig := NewDevelopmentConfig().
		WithAgentName("valid-agent").
		WithJWTAuth("HS256", "valid-secret", 15*time.Minute, 24*time.Hour)
	
	if err := goodConfig.Validate(); err != nil {
		fmt.Printf("Unexpected validation error: %v\n", err)
	} else {
		fmt.Println("Configuration validation passed!")
	}
}

// =============================================================================
// Migration Examples
// =============================================================================

// ExampleMigrationPath demonstrates how to migrate from old to new configuration
func ExampleMigrationPath() {
	// Step 1: Start with existing configuration (old way)
	legacyConfig := &AgentConfig{
		Name:        "legacy-app",
		Description: "Legacy application",
		// ... existing fields
	}
	
	// Step 2: Validate existing configuration with new validation
	if err := legacyConfig.Validate(); err != nil {
		log.Printf("Legacy config needs fixes: %v", err)
	}
	
	// Step 3: Gradually adopt new patterns
	// Can start using individual helper functions
	httpConfig := QuickHTTPConfig(30 * time.Second)
	jwtConfig := QuickJWTConfig("migrated-secret", 15*time.Minute, 24*time.Hour)
	
	// Step 4: Eventually move to full builder pattern
	newConfig := NewConfigBuilder().
		WithAgentName(legacyConfig.Name).
		WithAgentDescription(legacyConfig.Description).
		WithHTTP(httpConfig).
		WithJWTAuth(jwtConfig.SigningMethod, jwtConfig.SecretKey, 
			jwtConfig.AccessTokenTTL, jwtConfig.RefreshTokenTTL)
	
	agentConfig, _, _, _, _, err := newConfig.Build()
	if err != nil {
		log.Printf("Migration failed: %v", err)
		return
	}
	
	fmt.Printf("Successfully migrated: %s\n", agentConfig.Name)
}

// =============================================================================
// Performance Comparison
// =============================================================================

// ExamplePerformanceComparison demonstrates the efficiency of new approach
func ExamplePerformanceComparison() {
	// Old way - lots of manual setup and potential for errors
	start := time.Now()
	for i := 0; i < 1000; i++ {
		// Simulate old configuration creation
		_ = &AgentConfig{
			Name: fmt.Sprintf("agent-%d", i),
			// ... many fields to set manually
		}
	}
	oldWayDuration := time.Since(start)
	
	// New way - quick and less error-prone
	start = time.Now()
	for i := 0; i < 1000; i++ {
		_, _, _, _, _, err := NewDevelopmentConfig().
			WithAgentName(fmt.Sprintf("agent-%d", i)).
			Build()
		if err != nil {
			log.Printf("Config %d failed: %v", i, err)
		}
	}
	newWayDuration := time.Since(start)
	
	fmt.Printf("Performance Comparison:\n")
	fmt.Printf("  Old way: %v\n", oldWayDuration)
	fmt.Printf("  New way: %v\n", newWayDuration)
	fmt.Printf("  Difference: %v\n", newWayDuration-oldWayDuration)
}