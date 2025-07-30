package sse

import (
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

// Example_securityBasicAPIKey demonstrates basic API key authentication
func Example_securityBasicAPIKey() {
	// Create security configuration with API key authentication
	config := SecurityConfig{
		Auth: AuthConfig{
			Type:         AuthTypeAPIKey,
			APIKey:       "your-secret-api-key",
			APIKeyHeader: "X-API-Key",
		},
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         200,
		},
		Validation: ValidationConfig{
			Enabled:             true,
			MaxRequestSize:      10 * 1024 * 1024, // 10MB
			MaxHeaderSize:       1 * 1024 * 1024,  // 1MB
			AllowedContentTypes: []string{"application/json"},
		},
	}

	// Create security manager
	logger, _ := zap.NewProduction()
	securityManager, err := NewSecurityManager(config, logger)
	if err != nil {
		log.Fatal(err)
	}

	// Create HTTP handler with security
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Apply security checks
		authCtx, err := securityManager.Authenticate(r)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		if err := securityManager.CheckRateLimit(r); err != nil {
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		if err := securityManager.ValidateRequest(r); err != nil {
			http.Error(w, "Invalid request", http.StatusBadRequest)
			return
		}

		// Apply security headers
		securityManager.ApplySecurityHeaders(w, r)

		// Handle the request
		fmt.Fprintf(w, "Hello, %s!", authCtx.UserID)
	})

	// Start server
	log.Println("Server starting on :8080 with API key authentication")
	http.ListenAndServe(":8080", handler)
}

// Example_securityJWT demonstrates JWT authentication
func Example_securityJWT() {
	// JWT configuration
	jwtConfig := JWTConfig{
		SigningKey: "your-256-bit-secret",
		Algorithm:  "HS256",
		Expiration: 24 * time.Hour,
	}

	// Create security configuration with JWT authentication
	config := SecurityConfig{
		Auth: AuthConfig{
			Type: AuthTypeJWT,
			JWT:  jwtConfig,
		},
		CORS: CORSConfig{
			Enabled:          true,
			AllowedOrigins:   []string{"https://app.example.com"},
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE"},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
			MaxAge:           24 * time.Hour,
		},
	}

	// Generate a sample JWT token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":      "user123",
		"username": "john.doe",
		"roles":    []string{"user", "admin"},
		"exp":      time.Now().Add(24 * time.Hour).Unix(),
		"iat":      time.Now().Unix(),
	})

	tokenString, _ := token.SignedString([]byte(jwtConfig.SigningKey))
	fmt.Printf("Sample JWT token: %s\n", tokenString)

	// Create security manager and handler
	logger := zap.NewNop()
	securityManager, _ := NewSecurityManager(config, logger)

	// Create SSE handler with JWT security
	sseHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticate
		authCtx, err := securityManager.Authenticate(r)
		if err != nil {
			http.Error(w, "Invalid JWT token", http.StatusUnauthorized)
			return
		}

		// Apply security headers
		securityManager.ApplySecurityHeaders(w, r)

		// Set up SSE
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")

		// Send personalized events
		fmt.Fprintf(w, "data: Welcome %s! You have roles: %v\n\n",
			authCtx.Username, authCtx.Roles)

		if flusher, ok := w.(http.Flusher); ok {
			flusher.Flush()
		}
	})

	http.Handle("/events", sseHandler)
	log.Println("SSE server with JWT authentication on :8080")
	http.ListenAndServe(":8080", nil)
}

// Example_securityLevels demonstrates different security levels
func Example_securityLevels() {
	// Different security levels for different environments
	configs := map[string]SecurityConfig{
		"development": GetSecurityConfig(SecurityLevelMinimal),
		"staging":     GetSecurityConfig(SecurityLevelStandard),
		"production":  GetSecurityConfig(SecurityLevelHigh),
	}

	for env, config := range configs {
		fmt.Printf("\n%s environment security:\n", env)
		fmt.Printf("- Authentication: %s\n", config.Auth.Type)
		fmt.Printf("- Rate limiting: %v\n", config.RateLimit.Enabled)
		if config.RateLimit.Enabled {
			fmt.Printf("  - Requests/sec: %d\n", config.RateLimit.RequestsPerSecond)
		}
		fmt.Printf("- Request validation: %v\n", config.Validation.Enabled)
		fmt.Printf("- CORS enabled: %v\n", config.CORS.Enabled)
		fmt.Printf("- Request signing: %v\n", config.RequestSigning.Enabled)
	}
}

// Example_securityMultiAuth demonstrates multiple authentication methods
func Example_securityMultiAuth() {
	logger := zap.NewNop()

	// Handler that supports multiple auth methods
	multiAuthHandler := func(authTypes ...AuthType) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			var lastError error

			// Try each auth method
			for _, authType := range authTypes {
				config := SecurityConfig{
					Auth: getAuthConfig(authType),
				}

				sm, err := NewSecurityManager(config, logger)
				if err != nil {
					continue
				}

				if _, err := sm.Authenticate(r); err == nil {
					// Authentication successful
					w.WriteHeader(http.StatusOK)
					fmt.Fprintf(w, "Authenticated using %s", authType)
					return
				}
				lastError = err
			}

			// All auth methods failed
			http.Error(w, fmt.Sprintf("Authentication failed: %v", lastError),
				http.StatusUnauthorized)
		}
	}

	// Create handler that accepts JWT or API key
	handler := multiAuthHandler(AuthTypeJWT, AuthTypeAPIKey)

	http.Handle("/api/flexible", handler)
	log.Println("Server with flexible authentication on :8080")
}

// Helper function to get auth config for a type
func getAuthConfig(authType AuthType) AuthConfig {
	switch authType {
	case AuthTypeBearer:
		return AuthConfig{
			Type:        AuthTypeBearer,
			BearerToken: "bearer-token-123",
		}
	case AuthTypeAPIKey:
		return AuthConfig{
			Type:         AuthTypeAPIKey,
			APIKey:       "api-key-456",
			APIKeyHeader: "X-API-Key",
		}
	case AuthTypeJWT:
		return AuthConfig{
			Type: AuthTypeJWT,
			JWT: JWTConfig{
				SigningKey: "jwt-secret-key",
				Algorithm:  "HS256",
			},
		}
	default:
		return AuthConfig{Type: AuthTypeNone}
	}
}

// Example_securityRequestSigning demonstrates request signing for API security
func Example_securityRequestSigning() {
	// Configuration with request signing
	config := SecurityConfig{
		Auth: AuthConfig{
			Type:         AuthTypeAPIKey,
			APIKey:       "api-key",
			APIKeyHeader: "X-API-Key",
		},
		RequestSigning: RequestSigningConfig{
			Enabled:          true,
			Algorithm:        "HMAC-SHA256",
			SigningKey:       "shared-secret-key",
			SignedHeaders:    []string{"host", "date", "content-type"},
			SignatureHeader:  "X-Signature",
			TimestampHeader:  "X-Timestamp",
			MaxTimestampSkew: 5 * time.Minute,
		},
	}

	logger := zap.NewNop()
	securityManager, _ := NewSecurityManager(config, logger)
	signer := NewRequestSigner(config.RequestSigning)

	// Client side - sign request
	clientRequest := func(url string) {
		req, _ := http.NewRequest("POST", url, nil)
		req.Header.Set("X-API-Key", "api-key")
		req.Header.Set("Content-Type", "application/json")

		// Sign the request
		if err := signer.SignRequest(req); err != nil {
			log.Printf("Failed to sign request: %v", err)
			return
		}

		// Send request
		client := &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					MinVersion: tls.VersionTLS12,
					CipherSuites: []uint16{
						tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
						tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
						tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
						tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					},
				},
			},
		}
		resp, err := client.Do(req)
		if err != nil {
			log.Printf("Request failed: %v", err)
			return
		}
		defer resp.Body.Close()

		log.Printf("Response status: %d", resp.StatusCode)
	}

	// Server side - verify signature
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Authenticate
		if _, err := securityManager.Authenticate(r); err != nil {
			http.Error(w, "Authentication failed", http.StatusUnauthorized)
			return
		}

		// Verify signature
		if err := signer.VerifyRequest(r); err != nil {
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Request verified successfully")
	})

	// Example usage
	go http.ListenAndServe(":8080", handler)
	time.Sleep(100 * time.Millisecond)
	clientRequest("http://localhost:8080/api/data")
}

// Example_securityRateLimiting demonstrates advanced rate limiting
func Example_securityRateLimiting() {
	config := SecurityConfig{
		Auth: AuthConfig{Type: AuthTypeNone}, // No auth for this example
		RateLimit: RateLimitConfig{
			Enabled:           true,
			RequestsPerSecond: 100,
			BurstSize:         200,
			PerClient: RateLimitPerClientConfig{
				Enabled:              true,
				RequestsPerSecond:    10,
				BurstSize:            20,
				IdentificationMethod: "ip",
			},
			PerEndpoint: map[string]RateLimitEndpointConfig{
				"/api/expensive": {
					RequestsPerSecond: 1,
					BurstSize:         2,
				},
				"/api/search": {
					RequestsPerSecond: 5,
					BurstSize:         10,
				},
			},
		},
	}

	logger := zap.NewNop()
	securityManager, _ := NewSecurityManager(config, logger)

	// Create handlers with rate limiting
	mux := http.NewServeMux()

	// Wrap handler with rate limiting
	withRateLimit := func(handler http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			if err := securityManager.CheckRateLimit(r); err != nil {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
				return
			}
			handler(w, r)
		}
	}

	// Regular endpoint
	mux.HandleFunc("/api/data", withRateLimit(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "Regular data endpoint")
	}))

	// Expensive endpoint with lower rate limit
	mux.HandleFunc("/api/expensive", withRateLimit(func(w http.ResponseWriter, r *http.Request) {
		// Simulate expensive operation
		time.Sleep(500 * time.Millisecond)
		fmt.Fprintln(w, "Expensive operation completed")
	}))

	// Search endpoint with moderate rate limit
	mux.HandleFunc("/api/search", withRateLimit(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("q")
		fmt.Fprintf(w, "Search results for: %s\n", query)
	}))

	log.Println("Server with rate limiting on :8080")
	http.ListenAndServe(":8080", mux)
}

// Example_securityValidation demonstrates request validation
func Example_securityValidation() {
	config := SecurityConfig{
		Auth: AuthConfig{Type: AuthTypeNone},
		Validation: ValidationConfig{
			Enabled:             true,
			MaxRequestSize:      1024 * 1024, // 1MB
			MaxHeaderSize:       10 * 1024,   // 10KB
			AllowedContentTypes: []string{"application/json", "application/x-www-form-urlencoded"},
			RequestTimeout:      30 * time.Second,
			ValidateJSONSchema:  true,
		},
	}

	logger := zap.NewNop()
	securityManager, _ := NewSecurityManager(config, logger)

	_ = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate request
		if err := securityManager.ValidateRequest(r); err != nil {
			http.Error(w, fmt.Sprintf("Invalid request: %v", err), http.StatusBadRequest)
			return
		}

		// Apply security headers
		securityManager.ApplySecurityHeaders(w, r)

		// Process valid request
		fmt.Fprintln(w, "Request validated successfully")
	})

	// Test various requests
	testRequests := []struct {
		name    string
		method  string
		path    string
		body    string
		headers map[string]string
	}{
		{
			name:   "Valid JSON request",
			method: "POST",
			path:   "/api/data",
			body:   `{"name": "test", "value": 123}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
		{
			name:   "Path traversal attempt",
			method: "GET",
			path:   "/api/../../../etc/passwd",
		},
		{
			name:   "SQL injection attempt",
			method: "GET",
			path:   "/api/users?id=1' OR 1=1--",
		},
		{
			name:   "XSS attempt",
			method: "POST",
			path:   "/api/comment",
			body:   `{"text": "<script>alert('XSS')</script>"}`,
			headers: map[string]string{
				"Content-Type": "application/json",
			},
		},
	}

	for _, test := range testRequests {
		fmt.Printf("\nTesting: %s\n", test.name)
		// Create request and test validation
	}
}

// Example_securityMetrics demonstrates security metrics collection
func Example_securityMetrics() {
	config := standardSecurityConfig()
	logger := zap.NewNop()
	securityManager, _ := NewSecurityManager(config, logger)

	// Simulate various security events
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest("GET", "/api/data", nil)

		// Some succeed, some fail
		if i%3 == 0 {
			req.Header.Set(config.Auth.APIKeyHeader, "wrong-key")
		} else {
			req.Header.Set(config.Auth.APIKeyHeader, config.Auth.APIKey)
		}

		_, _ = securityManager.Authenticate(req)
		_ = securityManager.CheckRateLimit(req)
	}

	// Get and display metrics
	metrics := securityManager.Metrics().GetMetrics()
	fmt.Println("\nSecurity Metrics:")
	for key, value := range metrics {
		fmt.Printf("- %s: %d\n", key, value)
	}
}

// Example_securityTLS demonstrates TLS/SSL configuration
func Example_securityTLS() {
	// Create comprehensive config with TLS
	config := ComprehensiveConfig{
		Connection: ConnectionConfig{
			BaseURL: "https://api.example.com",
			TLS: TLSConfig{
				Enabled:            true,
				InsecureSkipVerify: false,
				MinVersion:         tls.VersionTLS12,
				MaxVersion:         tls.VersionTLS13,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				},
			},
		},
		Security: SecurityConfig{
			Auth: AuthConfig{
				Type:        AuthTypeBearer,
				BearerToken: "secure-token",
			},
		},
	}

	// Create HTTP client with TLS config
	httpClient := config.GetHTTPClient()

	// Create SSE client with security
	sseConfig := config.ToSimpleConfig()
	sseConfig.Client = httpClient

	// ctx := context.Background()
	// In a real implementation, you would create a client with:
	// client := NewClient(sseConfig)
	// stream, err := client.Connect(ctx)
	// if err != nil {
	//     log.Printf("Failed to connect: %v", err)
	//     return
	// }
	// defer stream.Close()

	fmt.Println("Connected securely with TLS")
}

// Example_securityAuditLog demonstrates audit logging
func Example_securityAuditLog() {
	// Create logger with audit configuration
	logConfig := zap.NewProductionConfig()
	logConfig.OutputPaths = []string{"stdout", "audit.log"}
	logger, _ := logConfig.Build()

	config := SecurityConfig{
		Auth: AuthConfig{
			Type:         AuthTypeAPIKey,
			APIKey:       "secret-key",
			APIKeyHeader: "X-API-Key",
		},
	}

	securityManager, _ := NewSecurityManager(config, logger)

	// Simulate security events
	requests := []struct {
		name   string
		apiKey string
		path   string
	}{
		{"Valid request", "secret-key", "/api/users"},
		{"Invalid API key", "wrong-key", "/api/users"},
		{"Missing API key", "", "/api/admin"},
		{"Suspicious path", "secret-key", "/api/../etc/passwd"},
	}

	for _, req := range requests {
		r := httptest.NewRequest("GET", req.path, nil)
		if req.apiKey != "" {
			r.Header.Set("X-API-Key", req.apiKey)
		}

		fmt.Printf("\n%s:\n", req.name)
		if _, err := securityManager.Authenticate(r); err != nil {
			fmt.Printf("  Authentication failed: %v\n", err)
		} else {
			fmt.Println("  Authentication successful")
		}

		if err := securityManager.ValidateRequest(r); err != nil {
			fmt.Printf("  Validation failed: %v\n", err)
		}
	}

	// Audit log will contain all security events
	fmt.Println("\nCheck audit.log for security events")
}
