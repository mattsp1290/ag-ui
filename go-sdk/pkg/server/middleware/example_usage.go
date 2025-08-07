package middleware

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// ExampleServer demonstrates how to use the comprehensive middleware system
type ExampleServer struct {
	logger *zap.Logger
	chain  *Chain
}

// NewExampleServer creates a new example server with all middleware configured
func NewExampleServer(logger *zap.Logger) (*ExampleServer, error) {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return nil, fmt.Errorf("failed to create logger: %w", err)
		}
	}

	server := &ExampleServer{
		logger: logger,
		chain:  NewChain(logger),
	}

	if err := server.setupMiddleware(); err != nil {
		return nil, fmt.Errorf("failed to setup middleware: %w", err)
	}

	return server, nil
}

// setupMiddleware configures all middleware in the correct order
func (es *ExampleServer) setupMiddleware() error {
	// 1. Request ID middleware (highest priority)
	requestIDMiddleware := RequestIDMiddleware(es.logger)
	es.chain.Use(&middlewareAdapter{
		fn:       requestIDMiddleware,
		name:     "request-id",
		priority: 200,
	})

	// 2. CORS middleware (before auth to handle preflight)
	corsConfig := &CORSConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 90,
			Name:     "cors",
		},
		AllowedOrigins: []string{
			"https://app.example.com",
			"https://admin.example.com",
			"*.dev.example.com",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{
			"Content-Type", "Authorization", "X-API-Key", "X-Requested-With",
		},
		AllowCredentials: true,
		MaxAge:           86400,
		Strict:           true,
	}

	corsMiddleware, err := NewCORSMiddleware(corsConfig, es.logger)
	if err != nil {
		return fmt.Errorf("failed to create CORS middleware: %w", err)
	}
	es.chain.Use(corsMiddleware)

	// 3. Rate limiting middleware (before auth to prevent abuse)
	rateLimitConfig := &RateLimitConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 80,
			Name:     "ratelimit",
		},
		Algorithm:         TokenBucket,
		Scope:             ScopeIP,
		RequestsPerMinute: 1000,
		BurstSize:         100,
		WindowSize:        time.Minute,

		// Endpoint-specific limits
		EndpointLimits: map[string]*EndpointRateLimit{
			"/api/auth/login": {
				Path:              "/api/auth/login",
				Method:            "POST",
				RequestsPerMinute: 5,
				BurstSize:         2,
				WindowSize:        time.Minute,
			},
		},

		IncludeHeaders:   true,
		RetryAfterHeader: true,
		SkipPaths:        []string{"/health", "/metrics"},
	}

	rateLimitMiddleware, err := NewRateLimitMiddleware(rateLimitConfig, es.logger)
	if err != nil {
		return fmt.Errorf("failed to create rate limit middleware: %w", err)
	}
	es.chain.Use(rateLimitMiddleware)

	// 4. Authentication middleware
	authConfig := &AuthConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 100,
			Name:     "auth",
		},
		Method: AuthMethodJWT,
		JWT: JWTConfig{
			SigningMethod: "HS256",
			SecretKeyEnv:  "JWT_SECRET_KEY", // Name of the environment variable containing the JWT secret key
			TokenHeader:   "Authorization",
			TokenPrefix:   "Bearer ",
			Issuer:        "ag-ui-server",
			Audience:      []string{"ag-ui-api"},
			LeewayTime:    time.Minute,
		},
		OptionalPaths:   []string{"/public", "/health", "/metrics"},
		ExcludedPaths:   []string{"/favicon.ico"},
		SecureErrorMode: true,
	}

	authMiddleware, err := NewAuthMiddleware(authConfig, es.logger)
	if err != nil {
		return fmt.Errorf("failed to create auth middleware: %w", err)
	}
	es.chain.Use(authMiddleware)

	// 5. Metrics middleware
	metricsCollector := NewInMemoryMetricsCollector()
	metricsConfig := &MetricsConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 20,
			Name:     "metrics",
		},
		Collector:             metricsCollector,
		EnableRequestMetrics:  true,
		EnableResponseMetrics: true,
		EnableDurationMetrics: true,
		EnableActiveRequests:  true,
		IncludeMethod:         true,
		IncludeStatus:         true,
		IncludePath:           true,
		IncludeUserID:         true,
		NormalizePaths:        true,
		PathReplacements: map[string]string{
			"/api/v1": "/api/{version}",
			"/api/v2": "/api/{version}",
		},
		ExcludePaths: []string{"/health", "/metrics"},
		CustomCounters: []CustomMetricConfig{
			{
				Name:      "http_errors_total",
				Help:      "Total HTTP errors",
				Labels:    []string{"method", "status"},
				ValueFrom: "constant",
				Condition: "error",
			},
		},
	}

	metricsMiddleware, err := NewMetricsMiddleware(metricsConfig, es.logger)
	if err != nil {
		return fmt.Errorf("failed to create metrics middleware: %w", err)
	}
	es.chain.Use(metricsMiddleware)

	// 6. Logging middleware (lowest priority to catch everything)
	loggingConfig := &LoggingConfig{
		BaseConfig: BaseConfig{
			Enabled:  true,
			Priority: 10,
			Name:     "logging",
		},
		Level:                LogLevelInfo,
		Format:               LogFormatJSON,
		IncludeClientIP:      true,
		IncludeUserAgent:     true,
		IncludeUserID:        true,
		IncludeRequestID:     true,
		IncludeTraceID:       true,
		LogSlowRequests:      true,
		SlowRequestThreshold: time.Second,
		SanitizeHeaders: []string{
			"authorization", "cookie", "set-cookie", "x-api-key",
		},
		ExcludePaths: []string{"/health", "/metrics"},
	}

	loggingMiddleware, err := NewLoggingMiddleware(loggingConfig, es.logger)
	if err != nil {
		return fmt.Errorf("failed to create logging middleware: %w", err)
	}
	es.chain.Use(loggingMiddleware)

	return nil
}

// GetHandler returns the configured middleware chain handler
func (es *ExampleServer) GetHandler() http.Handler {
	mux := http.NewServeMux()

	// API routes
	mux.HandleFunc("/api/users", es.handleUsers)
	mux.HandleFunc("/api/auth/login", es.handleLogin)
	mux.HandleFunc("/api/protected", es.handleProtected)

	// Public routes
	mux.HandleFunc("/public/info", es.handlePublicInfo)

	// Health and metrics
	mux.HandleFunc("/health", es.handleHealth)
	mux.HandleFunc("/metrics", es.handleMetrics)

	// Apply middleware chain
	return es.chain.Handler(mux)
}

// Example handlers

func (es *ExampleServer) handleUsers(w http.ResponseWriter, r *http.Request) {
	// This endpoint requires authentication
	user, ok := GetAuthUser(r.Context())
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	response := map[string]interface{}{
		"users": []map[string]string{
			{"id": "1", "name": "John Doe"},
			{"id": "2", "name": "Jane Smith"},
		},
		"authenticated_user": user.ID,
		"request_id":         GetRequestID(r.Context()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (es *ExampleServer) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// This is a simplified login - in practice, validate credentials
	response := map[string]interface{}{
		"token":      "eyJ0eXAiOiJKV1QiLCJhbGciOiJIUzI1NiJ9...", // Example JWT
		"user_id":    "user_123",
		"expires_in": 3600,
		"request_id": GetRequestID(r.Context()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (es *ExampleServer) handleProtected(w http.ResponseWriter, r *http.Request) {
	user, ok := GetAuthUser(r.Context())
	if !ok {
		http.Error(w, "Authentication required", http.StatusUnauthorized)
		return
	}

	response := map[string]interface{}{
		"message":    "This is a protected endpoint",
		"user_id":    user.ID,
		"user_roles": user.Roles,
		"request_id": GetRequestID(r.Context()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (es *ExampleServer) handlePublicInfo(w http.ResponseWriter, r *http.Request) {
	// This endpoint doesn't require authentication
	response := map[string]interface{}{
		"message":    "This is public information",
		"timestamp":  time.Now().Unix(),
		"request_id": GetRequestID(r.Context()),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (es *ExampleServer) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Unix(),
		"version":   "1.0.0",
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

func (es *ExampleServer) handleMetrics(w http.ResponseWriter, r *http.Request) {
	// Get metrics from the metrics middleware
	// This is a simplified version - in practice, you'd get the collector
	// from the middleware and expose metrics in Prometheus format

	metrics := map[string]interface{}{
		"http_requests_total":       100,
		"http_request_duration_avg": 0.125,
		"active_requests":           5,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// middlewareAdapter adapts MiddlewareFunc to Middleware interface
type middlewareAdapter struct {
	fn       MiddlewareFunc
	name     string
	priority int
}

func (ma *middlewareAdapter) Handler(next http.Handler) http.Handler {
	return ma.fn(next)
}

func (ma *middlewareAdapter) Name() string {
	return ma.name
}

func (ma *middlewareAdapter) Priority() int {
	return ma.priority
}

func (ma *middlewareAdapter) Config() interface{} {
	return nil
}

func (ma *middlewareAdapter) Cleanup() error {
	return nil
}

// ExampleWithCustomMiddleware demonstrates how to add custom middleware
func ExampleWithCustomMiddleware() http.Handler {
	logger, _ := zap.NewProduction()

	// Create chain
	chain := NewChain(logger)

	// Add custom middleware
	customMiddleware := &CustomSecurityMiddleware{
		config: &CustomSecurityConfig{
			EnableCSP:           true,
			CSPPolicy:           "default-src 'self'",
			EnableXFrameOptions: true,
			XFrameOptions:       "DENY",
		},
		logger: logger,
	}
	chain.Use(customMiddleware)

	// Add standard middleware
	corsMiddleware, _ := NewCORSMiddleware(DefaultCORSConfig(), logger)
	chain.Use(corsMiddleware)

	// Create handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Hello with custom middleware!"))
	})

	return chain.Handler(handler)
}

// CustomSecurityMiddleware demonstrates a custom middleware implementation
type CustomSecurityMiddleware struct {
	config *CustomSecurityConfig
	logger *zap.Logger
}

type CustomSecurityConfig struct {
	BaseConfig          `json:",inline" yaml:",inline"`
	EnableCSP           bool   `json:"enable_csp" yaml:"enable_csp"`
	CSPPolicy           string `json:"csp_policy" yaml:"csp_policy"`
	EnableXFrameOptions bool   `json:"enable_x_frame_options" yaml:"enable_x_frame_options"`
	XFrameOptions       string `json:"x_frame_options" yaml:"x_frame_options"`
}

func (csm *CustomSecurityMiddleware) Handler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Add security headers
		if csm.config.EnableCSP {
			w.Header().Set("Content-Security-Policy", csm.config.CSPPolicy)
		}

		if csm.config.EnableXFrameOptions {
			w.Header().Set("X-Frame-Options", csm.config.XFrameOptions)
		}

		// Add other security headers
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")

		next.ServeHTTP(w, r)
	})
}

func (csm *CustomSecurityMiddleware) Name() string {
	return "custom-security"
}

func (csm *CustomSecurityMiddleware) Priority() int {
	return 95 // High priority
}

func (csm *CustomSecurityMiddleware) Config() interface{} {
	return csm.config
}

func (csm *CustomSecurityMiddleware) Cleanup() error {
	return nil
}

// ExampleConditionalMiddleware demonstrates conditional middleware execution
func ExampleConditionalMiddleware() http.Handler {
	logger, _ := zap.NewProduction()
	chain := NewChain(logger)

	// Create auth middleware
	authConfig := &AuthConfig{
		BaseConfig: BaseConfig{Enabled: true, Name: "auth"},
		Method:     AuthMethodJWT,
		JWT: JWTConfig{
			SigningMethod: "HS256",
			SecretKeyEnv:  "JWT_SECRET_KEY", // Name of the environment variable containing the JWT secret key
		},
	}
	authMiddleware, _ := NewAuthMiddleware(authConfig, logger)

	// Only apply auth to API routes
	condition := func(r *http.Request) bool {
		return r.URL.Path != "/health" && r.URL.Path != "/public"
	}

	conditionalAuth := NewConditionalMiddleware(authMiddleware, condition, logger)
	chain.Use(conditionalAuth)

	// Handler
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if user, ok := GetAuthUser(r.Context()); ok {
			fmt.Fprintf(w, "Authenticated user: %s", user.ID)
		} else {
			fmt.Fprintf(w, "Public access")
		}
	})

	return chain.Handler(handler)
}

// ExampleWithManager demonstrates using the middleware manager
func ExampleWithManager() map[string]http.Handler {
	logger, _ := zap.NewProduction()
	manager := NewManager(logger)

	// Create API chain with full middleware
	apiChain := manager.CreateChain("api")

	// API middleware
	corsMiddleware, _ := NewCORSMiddleware(DefaultCORSConfig(), logger)
	authMiddleware, _ := RequireAuth(AuthMethodJWT, logger)
	rateLimitMiddleware, _ := NewRateLimitMiddleware(DefaultRateLimitConfig(), logger)

	apiChain.Use(corsMiddleware, authMiddleware, rateLimitMiddleware)

	// Create public chain with minimal middleware
	publicChain := manager.CreateChain("public")

	// Public middleware (no auth)
	publicChain.Use(corsMiddleware)

	// Create handlers
	apiHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _ := GetAuthUser(r.Context())
		fmt.Fprintf(w, "API response for user: %s", user.ID)
	})

	publicHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "Public content")
	})

	return map[string]http.Handler{
		"api":    apiChain.Handler(apiHandler),
		"public": publicChain.Handler(publicHandler),
	}
}

// ExampleErrorHandling demonstrates error handling in middleware
func ExampleErrorHandling() http.Handler {
	logger, _ := zap.NewProduction()

	// Error recovery middleware
	recovery := MiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						zap.Any("error", err),
						zap.String("path", r.URL.Path),
					)

					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				}
			}()

			next.ServeHTTP(w, r)
		})
	})

	chain := NewChain(logger)
	chain.Use(&middlewareAdapter{
		fn:       recovery,
		name:     "recovery",
		priority: 1000, // Highest priority
	})

	// Handler that might panic
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/panic" {
			panic("Example panic")
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	return chain.Handler(handler)
}

// ExampleHTTPServer demonstrates running a complete HTTP server
func ExampleHTTPServer() error {
	logger, err := zap.NewProduction()
	if err != nil {
		return err
	}
	defer logger.Sync()

	// Create example server
	server, err := NewExampleServer(logger)
	if err != nil {
		return err
	}

	// Get configured handler
	handler := server.GetHandler()

	// Start HTTP server
	logger.Info("Starting server on :8080")
	return http.ListenAndServe(":8080", handler)
}

// Example usage in main function:
//
// func main() {
//     if err := middleware.ExampleHTTPServer(); err != nil {
//         log.Fatal(err)
//     }
// }
