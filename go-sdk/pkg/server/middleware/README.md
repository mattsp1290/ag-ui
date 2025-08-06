# AG-UI Go SDK Middleware System

A comprehensive, pluggable middleware framework for HTTP servers with built-in implementations for authentication, CORS, logging, metrics, and rate limiting.

## Features

- **Framework-agnostic**: Works with any HTTP server framework
- **Pluggable architecture**: Easy to add custom middleware
- **High performance**: Optimized for hot paths with minimal overhead
- **Comprehensive logging**: Structured logging with configurable sanitization
- **Multiple authentication methods**: JWT, API keys, Basic auth, HMAC, Bearer tokens
- **Advanced rate limiting**: Token bucket, sliding window, and fixed window algorithms
- **CORS support**: Full CORS specification compliance with security-first approach
- **Metrics collection**: Built-in performance monitoring and custom metrics
- **Configuration-driven**: Extensive YAML/JSON configuration support

## Quick Start

```go
package main

import (
    "log"
    "net/http"
    
    "github.com/mattsp1290/ag-ui/go-sdk/pkg/server/middleware"
    "go.uber.org/zap"
)

func main() {
    logger, _ := zap.NewProduction()
    
    // Create middleware chain
    chain := middleware.NewChain(logger)
    
    // Add CORS middleware
    corsConfig := middleware.DefaultCORSConfig()
    corsMiddleware, _ := middleware.NewCORSMiddleware(corsConfig, logger)
    chain.Use(corsMiddleware)
    
    // Add authentication middleware
    authConfig := &middleware.AuthConfig{
        BaseConfig: middleware.BaseConfig{
            Enabled:  true,
            Priority: 100,
            Name:     "auth",
        },
        Method: middleware.AuthMethodJWT,
        JWT: middleware.JWTConfig{
            SigningMethod: "HS256",
            SecretKey:     "your-secret-key",
        },
    }
    authMiddleware, _ := middleware.NewAuthMiddleware(authConfig, logger)
    chain.Use(authMiddleware)
    
    // Add rate limiting
    rateLimitConfig := middleware.DefaultRateLimitConfig()
    rateLimitMiddleware, _ := middleware.NewRateLimitMiddleware(rateLimitConfig, logger)
    chain.Use(rateLimitMiddleware)
    
    // Add logging
    loggingConfig := middleware.DefaultLoggingConfig()
    loggingMiddleware, _ := middleware.NewLoggingMiddleware(loggingConfig, logger)
    chain.Use(loggingMiddleware)
    
    // Your application handler
    appHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
        w.Write([]byte("Hello, World!"))
    })
    
    // Apply middleware chain
    finalHandler := chain.Handler(appHandler)
    
    // Start server
    log.Println("Server starting on :8080")
    log.Fatal(http.ListenAndServe(":8080", finalHandler))
}
```

## Middleware Components

### 1. Core Middleware System

The core system provides the foundation for all middleware:

```go
// Middleware interface
type Middleware interface {
    Handler(next http.Handler) http.Handler
    Name() string
    Priority() int
    Config() interface{}
    Cleanup() error
}

// Chain for composing middleware
chain := middleware.NewChain(logger)
chain.Use(middleware1, middleware2, middleware3)
handler := chain.Handler(finalHandler)
```

### 2. Authentication Middleware

Supports multiple authentication methods:

#### JWT Authentication
```go
authConfig := &middleware.AuthConfig{
    Method: middleware.AuthMethodJWT,
    JWT: middleware.JWTConfig{
        SigningMethod: "HS256",
        SecretKey:     "your-secret-key",
        TokenHeader:   "Authorization",
        TokenPrefix:   "Bearer ",
        Issuer:        "your-app",
        Audience:      []string{"your-api"},
    },
}
```

#### API Key Authentication
```go
authConfig := &middleware.AuthConfig{
    Method: middleware.AuthMethodAPIKey,
    APIKey: middleware.APIKeyConfig{
        HeaderName: "X-API-Key",
        Keys: map[string]*middleware.APIKeyInfo{
            "your-api-key": {
                UserID:      "user123",
                Roles:       []string{"user"},
                Permissions: []string{"read", "write"},
            },
        },
    },
}
```

#### Basic Authentication
```go
authConfig := &middleware.AuthConfig{
    Method: middleware.AuthMethodBasic,
    BasicAuth: middleware.BasicAuthConfig{
        Realm: "API",
        Users: map[string]*middleware.BasicAuthUser{
            "admin": {
                PasswordHash: "$2a$10$...", // bcrypt hash
                UserID:       "admin123",
                Roles:        []string{"admin"},
            },
        },
    },
}
```

### 3. CORS Middleware

Full CORS specification support:

```go
corsConfig := &middleware.CORSConfig{
    AllowedOrigins: []string{
        "https://app.example.com",
        "*.dev.example.com",
    },
    AllowedMethods: []string{"GET", "POST", "PUT", "DELETE"},
    AllowedHeaders: []string{"Content-Type", "Authorization"},
    AllowCredentials: true,
    MaxAge: 86400,
    Strict: true, // Enforce strict CORS policies
}
```

### 4. Rate Limiting Middleware

Multiple algorithms and scoping options:

```go
rateLimitConfig := &middleware.RateLimitConfig{
    Algorithm:         middleware.TokenBucket,
    Scope:            middleware.ScopeIP,
    RequestsPerMinute: 1000,
    BurstSize:        100,
    
    // Per-endpoint limits
    EndpointLimits: map[string]*middleware.EndpointRateLimit{
        "/api/auth/login": {
            RequestsPerMinute: 5,
            BurstSize:        2,
        },
    },
    
    // Per-user limits
    UserLimits: map[string]*middleware.UserRateLimit{
        "premium_user": {
            RequestsPerMinute: 5000,
            BurstSize:        500,
        },
    },
}
```

### 5. Logging Middleware

Structured logging with sanitization:

```go
loggingConfig := &middleware.LoggingConfig{
    Level:  middleware.LogLevelInfo,
    Format: middleware.LogFormatJSON,
    
    // Include/exclude data
    IncludeRequestBody:  false,
    IncludeResponseBody: false,
    IncludeClientIP:     true,
    IncludeUserID:       true,
    
    // Sanitize sensitive data
    SanitizeHeaders: []string{"authorization", "cookie"},
    SanitizeQueries: []string{"token", "key", "password"},
    
    // Performance logging
    LogSlowRequests:      true,
    SlowRequestThreshold: time.Second,
}
```

### 6. Metrics Middleware

Performance monitoring and custom metrics:

```go
metricsConfig := &middleware.MetricsConfig{
    EnableRequestMetrics:  true,
    EnableResponseMetrics: true,
    EnableDurationMetrics: true,
    EnableActiveRequests:  true,
    
    // Labels
    IncludeMethod: true,
    IncludeStatus: true,
    IncludePath:   true,
    
    // Path normalization
    NormalizePaths: true,
    PathReplacements: map[string]string{
        "/api/v1": "/api/{version}",
    },
    
    // Custom metrics
    CustomCounters: []middleware.CustomMetricConfig{
        {
            Name:      "http_errors_total",
            Labels:    []string{"method", "status"},
            ValueFrom: "constant",
            Condition: "error",
        },
    },
}

// Get metrics collector for exporting
collector := metricsMiddleware.GetCollector()
```

## Configuration

### YAML Configuration

See [example_config.yaml](example_config.yaml) for a comprehensive configuration example.

### Environment Variables

Configuration supports environment variable substitution:

```yaml
auth:
  jwt:
    secret_key: "${JWT_SECRET}"
  hmac:
    secret_key: "${HMAC_SECRET}"
```

### Programmatic Configuration

All middleware can be configured programmatically:

```go
// Default configurations
corsConfig := middleware.DefaultCORSConfig()
authConfig := middleware.RequireAuth(middleware.AuthMethodJWT, logger)
rateLimitConfig := middleware.DefaultRateLimitConfig()
```

## Advanced Usage

### Custom Middleware

Implement the `Middleware` interface:

```go
type CustomMiddleware struct {
    config *CustomConfig
    logger *zap.Logger
}

func (cm *CustomMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Custom logic here
        next.ServeHTTP(w, r)
    })
}

func (cm *CustomMiddleware) Name() string { return "custom" }
func (cm *CustomMiddleware) Priority() int { return 50 }
func (cm *CustomMiddleware) Config() interface{} { return cm.config }
func (cm *CustomMiddleware) Cleanup() error { return nil }
```

### Conditional Middleware

Execute middleware based on conditions:

```go
condition := func(r *http.Request) bool {
    return strings.HasPrefix(r.URL.Path, "/api/")
}

conditionalAuth := middleware.NewConditionalMiddleware(
    authMiddleware, 
    condition, 
    logger,
)
chain.Use(conditionalAuth)
```

### Middleware Manager

Manage multiple chains:

```go
manager := middleware.NewManager(logger)

// Create different chains for different routes
apiChain := manager.CreateChain("api")
publicChain := manager.CreateChain("public")

// Configure chains differently
apiChain.Use(authMiddleware, rateLimitMiddleware)
publicChain.Use(corsMiddleware, loggingMiddleware)
```

### Context Integration

Access middleware data in handlers:

```go
func handler(w http.ResponseWriter, r *http.Request) {
    // Get authenticated user
    if user, ok := middleware.GetAuthUser(r.Context()); ok {
        log.Printf("User: %s", user.ID)
    }
    
    // Get request ID
    requestID := middleware.GetRequestID(r.Context())
    
    // Get user ID
    userID := middleware.GetUserID(r.Context())
}
```

## Performance Considerations

### Hot Path Optimization

- Precomputed maps for fast lookups
- Minimal allocations in request path
- Efficient string operations
- Connection pooling where applicable

### Memory Management

- Automatic cleanup of old rate limiters
- Bounded memory usage for metrics
- Efficient request/response buffering

### Sampling

For high-traffic applications, use sampling:

```go
metricsConfig.SampleRate = 0.1  // 10% sampling
rateLimitConfig.SampleRate = 1.0 // No sampling for rate limiting
```

## Security Features

### Authentication Security

- Secure error messages in production
- Token validation with proper key verification
- HMAC signature validation with replay protection
- Bcrypt for password hashing

### CORS Security

- No wildcard origins with credentials
- Strict mode for production
- Proper preflight handling
- Security-first header policies

### Rate Limiting Security

- IP blacklisting/whitelisting
- User-based rate limiting
- Endpoint-specific limits
- Protection against abuse patterns

### Logging Security

- Automatic sanitization of sensitive data
- Configurable header/query sanitization
- Secure error logging
- No sensitive data in logs by default

## Monitoring and Observability

### Metrics Export

```go
// HTTP endpoint for metrics
http.HandleFunc("/metrics", middleware.MetricsHandler(collector))

// Get metrics programmatically
metrics := collector.GetMetrics()
```

### Health Checks

```go
// Check middleware health
func healthCheck(w http.ResponseWriter, r *http.Request) {
    stats := map[string]interface{}{
        "rate_limiter_stats": rateLimitMiddleware.GetLimiterStats(),
        "active_requests":    metricsMiddleware.GetActiveRequests(),
    }
    json.NewEncoder(w).Encode(stats)
}
```

### Tracing Integration

```go
// Add trace ID to context
func tracingMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        traceID := r.Header.Get("X-Trace-ID")
        if traceID == "" {
            traceID = generateTraceID()
        }
        
        ctx := context.WithValue(r.Context(), "trace_id", traceID)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## Testing

### Unit Testing

```go
func TestAuthMiddleware(t *testing.T) {
    config := &middleware.AuthConfig{
        Method: middleware.AuthMethodJWT,
        JWT: middleware.JWTConfig{
            SigningMethod: "HS256",
            SecretKey:     "test-secret",
        },
    }
    
    authMiddleware, err := middleware.NewAuthMiddleware(config, zap.NewNop())
    require.NoError(t, err)
    
    // Test with valid token
    req := httptest.NewRequest("GET", "/protected", nil)
    req.Header.Set("Authorization", "Bearer "+validToken)
    
    rr := httptest.NewRecorder()
    handler := authMiddleware.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.WriteHeader(http.StatusOK)
    }))
    
    handler.ServeHTTP(rr, req)
    assert.Equal(t, http.StatusOK, rr.Code)
}
```

### Integration Testing

```go
func TestMiddlewareChain(t *testing.T) {
    logger := zap.NewNop()
    chain := middleware.NewChain(logger)
    
    // Add all middleware
    chain.Use(
        corsMiddleware,
        authMiddleware,
        rateLimitMiddleware,
        loggingMiddleware,
    )
    
    handler := chain.Handler(testHandler)
    
    // Test complete request flow
    req := httptest.NewRequest("POST", "/api/test", nil)
    req.Header.Set("Origin", "https://app.example.com")
    req.Header.Set("Authorization", "Bearer "+validToken)
    
    rr := httptest.NewRecorder()
    handler.ServeHTTP(rr, req)
    
    // Verify all middleware ran correctly
    assert.Equal(t, http.StatusOK, rr.Code)
    assert.Contains(t, rr.Header().Get("Access-Control-Allow-Origin"), "https://app.example.com")
}
```

## Migration from Existing Middleware

### From Standard HTTP Middleware

```go
// Old way
func oldMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // middleware logic
        next.ServeHTTP(w, r)
    })
}

// New way
type NewMiddleware struct{}

func (nm *NewMiddleware) Handler(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // same middleware logic
        next.ServeHTTP(w, r)
    })
}

// Or use adapter
chain.Use(middleware.MiddlewareFunc(oldMiddleware))
```

### From Gorilla Mux

```go
// Old Gorilla middleware
r.Use(gorillaMiddleware)

// New middleware chain
handler := chain.Handler(r)
```

## Troubleshooting

### Common Issues

1. **Authentication failing**: Check token format and signing key
2. **CORS errors**: Verify origin configuration and credentials setting
3. **Rate limiting too aggressive**: Adjust burst size and time windows
4. **High memory usage**: Enable cleanup and set appropriate TTLs
5. **Performance issues**: Enable sampling and optimize hot paths

### Debug Mode

Enable debug logging for troubleshooting:

```go
config.Debug = true
config.Level = middleware.LogLevelDebug
```

### Metrics Analysis

Monitor key metrics:
- Request rate and latency
- Authentication success/failure rates
- Rate limiting trigger frequency
- Error rates by endpoint

## Best Practices

1. **Security First**: Always use secure defaults in production
2. **Performance**: Profile and optimize hot paths
3. **Observability**: Implement comprehensive logging and metrics
4. **Configuration**: Use environment-specific configurations
5. **Testing**: Write comprehensive tests for all middleware
6. **Documentation**: Document custom middleware and configurations

## Contributing

Contributions are welcome! Please see [CONTRIBUTING.md](../../../CONTRIBUTING.md) for guidelines.

## License

This middleware system is part of the AG-UI Go SDK and follows the same license terms.