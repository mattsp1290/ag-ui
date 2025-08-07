package security

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Local type definitions to avoid circular imports
type Request struct {
	ID        string                 `json:"id"`
	Method    string                 `json:"method"`
	Path      string                 `json:"path"`
	Headers   map[string]string      `json:"headers"`
	Body      interface{}            `json:"body"`
	Metadata  map[string]interface{} `json:"metadata"`
	Timestamp time.Time              `json:"timestamp"`
}

type Response struct {
	ID         string                 `json:"id"`
	StatusCode int                    `json:"status_code"`
	Headers    map[string]string      `json:"headers"`
	Body       interface{}            `json:"body"`
	Error      error                  `json:"error,omitempty"`
	Metadata   map[string]interface{} `json:"metadata"`
	Timestamp  time.Time              `json:"timestamp"`
	Duration   time.Duration          `json:"duration"`
}

type NextHandler func(ctx context.Context, req *Request) (*Response, error)

// SecurityConfig represents security middleware configuration
type SecurityConfig struct {
	CORS            *CORSConfig            `json:"cors" yaml:"cors"`
	CSRF            *CSRFConfig            `json:"csrf" yaml:"csrf"`
	Headers         *SecurityHeadersConfig `json:"headers" yaml:"headers"`
	InputValidation *InputValidationConfig `json:"input_validation" yaml:"input_validation"`
	ThreatDetection *ThreatDetectionConfig `json:"threat_detection" yaml:"threat_detection"`
	SkipPaths       []string               `json:"skip_paths" yaml:"skip_paths"`
	SkipHealthCheck bool                   `json:"skip_health_check" yaml:"skip_health_check"`
}

// SecurityMiddleware implements comprehensive security middleware
type SecurityMiddleware struct {
	config         *SecurityConfig
	enabled        bool
	priority       int
	skipMap        map[string]bool
	corsHandler    *CORSHandler
	csrfHandler    *CSRFHandler
	headersHandler *HeadersHandler
	inputValidator *InputValidator
	threatDetector *ThreatDetector
	auditLogger    *SecurityAuditLogger
	mu             sync.RWMutex
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config *SecurityConfig) (*SecurityMiddleware, error) {
	if config == nil {
		config = &SecurityConfig{
			CORS: &CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			},
			Headers: &SecurityHeadersConfig{
				Enabled:             true,
				XFrameOptions:       "DENY",
				XContentTypeOptions: "nosniff",
				XXSSProtection:      "1; mode=block",
			},
			ThreatDetection: &ThreatDetectionConfig{
				Enabled:      true,
				SQLInjection: true,
				XSSDetection: true,
				LogThreats:   true,
			},
			SkipHealthCheck: true,
		}
	}

	skipMap := make(map[string]bool)
	for _, path := range config.SkipPaths {
		skipMap[path] = true
	}

	// Add common health check paths
	if config.SkipHealthCheck {
		skipMap["/health"] = true
		skipMap["/healthz"] = true
		skipMap["/ping"] = true
		skipMap["/ready"] = true
		skipMap["/live"] = true
	}

	// Initialize handlers
	corsHandler := NewCORSHandler(config.CORS)
	csrfHandler := NewCSRFHandler(config.CSRF)
	headersHandler := NewHeadersHandler(config.Headers)
	inputValidator := NewInputValidator(config.InputValidation)
	threatDetector, err := NewThreatDetector(config.ThreatDetection)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize threat detector: %w", err)
	}
	auditLogger := NewSecurityAuditLogger(true)

	sm := &SecurityMiddleware{
		config:         config,
		enabled:        true,
		priority:       200, // Very high priority for security
		skipMap:        skipMap,
		corsHandler:    corsHandler,
		csrfHandler:    csrfHandler,
		headersHandler: headersHandler,
		inputValidator: inputValidator,
		threatDetector: threatDetector,
		auditLogger:    auditLogger,
	}

	return sm, nil
}

// Name returns middleware name
func (sm *SecurityMiddleware) Name() string {
	return "security"
}

// Process processes the request through security middleware
func (sm *SecurityMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Skip security for configured paths
	if sm.skipMap[req.Path] {
		return next(ctx, req)
	}

	// Input validation
	if sm.inputValidator.Enabled() {
		if err := sm.inputValidator.ValidateInput(ctx, req); err != nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 400,
				Error:      fmt.Errorf("input validation failed: %w", err),
				Timestamp:  time.Now(),
			}, nil
		}
	}

	// Threat detection
	if sm.threatDetector.Enabled() {
		if threat, err := sm.threatDetector.DetectThreats(ctx, req); err != nil || threat != "" {
			if sm.threatDetector.ShouldLog() {
				sm.auditLogger.LogThreat(ctx, threat, req)
			}

			if sm.threatDetector.ShouldBlock() {
				return &Response{
					ID:         req.ID,
					StatusCode: 403,
					Error:      fmt.Errorf("security threat detected: %s", threat),
					Timestamp:  time.Now(),
				}, nil
			}
		}
	}

	// CSRF protection
	if sm.csrfHandler.Enabled() {
		if err := sm.csrfHandler.ValidateCSRF(ctx, req); err != nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 403,
				Error:      fmt.Errorf("CSRF validation failed: %w", err),
				Timestamp:  time.Now(),
			}, nil
		}
	}

	// Handle CORS preflight
	if sm.corsHandler.Enabled() && req.Method == "OPTIONS" {
		return sm.corsHandler.HandlePreflight(ctx, req)
	}

	// Process request through next middleware
	resp, err := next(ctx, req)
	if err != nil {
		return resp, err
	}

	// Add security headers to response
	if resp != nil {
		sm.headersHandler.AddSecurityHeaders(resp)
		sm.corsHandler.AddCORSHeaders(req, resp)
	}

	return resp, err
}

// Configure configures the middleware
func (sm *SecurityMiddleware) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		sm.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		sm.priority = priority
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (sm *SecurityMiddleware) Enabled() bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	return sm.enabled
}

// Priority returns the middleware priority
func (sm *SecurityMiddleware) Priority() int {
	return sm.priority
}

// GenerateCSRFToken generates a new CSRF token
func (sm *SecurityMiddleware) GenerateCSRFToken() string {
	return sm.csrfHandler.GenerateCSRFToken()
}

// CleanupExpiredTokens removes expired CSRF tokens
func (sm *SecurityMiddleware) CleanupExpiredTokens() {
	sm.csrfHandler.CleanupExpiredTokens()
}

// SecurityAuditLogger logs security events
type SecurityAuditLogger struct {
	enabled bool
}

// NewSecurityAuditLogger creates a new security audit logger
func NewSecurityAuditLogger(enabled bool) *SecurityAuditLogger {
	return &SecurityAuditLogger{
		enabled: enabled,
	}
}

// LogThreat logs a security threat
func (sal *SecurityAuditLogger) LogThreat(ctx context.Context, threat string, req *Request) {
	if !sal.enabled {
		return
	}

	fmt.Printf("SECURITY AUDIT: Threat detected - %s | Method: %s | Path: %s | Headers: %v\n",
		threat, req.Method, req.Path, req.Headers)
}

// LogSecurityEvent logs a general security event
func (sal *SecurityAuditLogger) LogSecurityEvent(ctx context.Context, event string, details map[string]interface{}) {
	if !sal.enabled {
		return
	}

	fmt.Printf("SECURITY AUDIT: %s | Details: %v\n", event, details)
}
