package security

import (
	"context"
	"fmt"
	"html"
	"regexp"
	"strings"
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

// CORSConfig represents CORS configuration
type CORSConfig struct {
	Enabled          bool     `json:"enabled" yaml:"enabled"`
	AllowedOrigins   []string `json:"allowed_origins" yaml:"allowed_origins"`
	AllowedMethods   []string `json:"allowed_methods" yaml:"allowed_methods"`
	AllowedHeaders   []string `json:"allowed_headers" yaml:"allowed_headers"`
	ExposedHeaders   []string `json:"exposed_headers" yaml:"exposed_headers"`
	AllowCredentials bool     `json:"allow_credentials" yaml:"allow_credentials"`
	MaxAge           int      `json:"max_age" yaml:"max_age"`
	OptionsSuccess   int      `json:"options_success" yaml:"options_success"`
}

// CSRFConfig represents CSRF protection configuration
type CSRFConfig struct {
	Enabled        bool     `json:"enabled" yaml:"enabled"`
	TokenHeader    string   `json:"token_header" yaml:"token_header"`
	TokenField     string   `json:"token_field" yaml:"token_field"`
	TokenLength    int      `json:"token_length" yaml:"token_length"`
	SecretKey      string   `json:"secret_key" yaml:"secret_key"`
	ExemptPaths    []string `json:"exempt_paths" yaml:"exempt_paths"`
	SafeMethods    []string `json:"safe_methods" yaml:"safe_methods"`
	ValidateOrigin bool     `json:"validate_origin" yaml:"validate_origin"`
}

// SecurityHeadersConfig represents security headers configuration
type SecurityHeadersConfig struct {
	Enabled                   bool   `json:"enabled" yaml:"enabled"`
	XFrameOptions             string `json:"x_frame_options" yaml:"x_frame_options"`
	XContentTypeOptions       string `json:"x_content_type_options" yaml:"x_content_type_options"`
	XXSSProtection            string `json:"x_xss_protection" yaml:"x_xss_protection"`
	ContentSecurityPolicy     string `json:"content_security_policy" yaml:"content_security_policy"`
	StrictTransportSecurity   string `json:"strict_transport_security" yaml:"strict_transport_security"`
	ReferrerPolicy            string `json:"referrer_policy" yaml:"referrer_policy"`
	PermissionsPolicy         string `json:"permissions_policy" yaml:"permissions_policy"`
	XPermittedCrossDomainPolicies string `json:"x_permitted_cross_domain_policies" yaml:"x_permitted_cross_domain_policies"`
}

// InputValidationConfig represents input validation configuration
type InputValidationConfig struct {
	Enabled              bool   `json:"enabled" yaml:"enabled"`
	MaxRequestSize       int64  `json:"max_request_size" yaml:"max_request_size"`
	MaxHeaderSize        int64  `json:"max_header_size" yaml:"max_header_size"`
	MaxQueryParams       int    `json:"max_query_params" yaml:"max_query_params"`
	MaxFormFields        int    `json:"max_form_fields" yaml:"max_form_fields"`
	AllowedContentTypes  []string `json:"allowed_content_types" yaml:"allowed_content_types"`
	BlockedPatterns      []string `json:"blocked_patterns" yaml:"blocked_patterns"`
	SanitizeHTML         bool   `json:"sanitize_html" yaml:"sanitize_html"`
	ValidateJSON         bool   `json:"validate_json" yaml:"validate_json"`
	StripControlChars    bool   `json:"strip_control_chars" yaml:"strip_control_chars"`
}

// ThreatDetectionConfig represents threat detection configuration
type ThreatDetectionConfig struct {
	Enabled           bool     `json:"enabled" yaml:"enabled"`
	SQLInjection      bool     `json:"sql_injection" yaml:"sql_injection"`
	XSSDetection      bool     `json:"xss_detection" yaml:"xss_detection"`
	PathTraversal     bool     `json:"path_traversal" yaml:"path_traversal"`
	CommandInjection  bool     `json:"command_injection" yaml:"command_injection"`
	BlockSuspicious   bool     `json:"block_suspicious" yaml:"block_suspicious"`
	LogThreats        bool     `json:"log_threats" yaml:"log_threats"`
	ThreatPatterns    []string `json:"threat_patterns" yaml:"threat_patterns"`
}

// SecurityMiddleware implements comprehensive security middleware
type SecurityMiddleware struct {
	config       *SecurityConfig
	enabled      bool
	priority     int
	skipMap      map[string]bool
	csrfTokens   map[string]time.Time
	threatRegexs []*regexp.Regexp
	mu           sync.RWMutex
}

// NewSecurityMiddleware creates a new security middleware
func NewSecurityMiddleware(config *SecurityConfig) (*SecurityMiddleware, error) {
	if config == nil {
		config = &SecurityConfig{
			CORS: &CORSConfig{
				Enabled:        true,
				AllowedOrigins: []string{"*"},
				AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
				AllowedHeaders: []string{"*"},
				MaxAge:         86400,
				OptionsSuccess: 204,
			},
			Headers: &SecurityHeadersConfig{
				Enabled:                 true,
				XFrameOptions:          "DENY",
				XContentTypeOptions:    "nosniff",
				XXSSProtection:         "1; mode=block",
				StrictTransportSecurity: "max-age=31536000; includeSubDomains",
				ReferrerPolicy:         "strict-origin-when-cross-origin",
			},
			InputValidation: &InputValidationConfig{
				Enabled:         true,
				MaxRequestSize:  10 * 1024 * 1024, // 10MB
				MaxHeaderSize:   8192,              // 8KB
				MaxQueryParams:  100,
				MaxFormFields:   100,
				SanitizeHTML:    true,
				ValidateJSON:    true,
				StripControlChars: true,
			},
			ThreatDetection: &ThreatDetectionConfig{
				Enabled:          true,
				SQLInjection:     true,
				XSSDetection:     true,
				PathTraversal:    true,
				CommandInjection: true,
				BlockSuspicious:  true,
				LogThreats:       true,
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

	sm := &SecurityMiddleware{
		config:     config,
		enabled:    true,
		priority:   200, // Very high priority for security
		skipMap:    skipMap,
		csrfTokens: make(map[string]time.Time),
	}

	// Initialize threat detection patterns
	if err := sm.initThreatPatterns(); err != nil {
		return nil, fmt.Errorf("failed to initialize threat patterns: %w", err)
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
	if sm.config.InputValidation != nil && sm.config.InputValidation.Enabled {
		if err := sm.validateInput(ctx, req); err != nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 400,
				Error:      fmt.Errorf("input validation failed: %w", err),
				Timestamp:  time.Now(),
			}, nil
		}
	}

	// Threat detection
	if sm.config.ThreatDetection != nil && sm.config.ThreatDetection.Enabled {
		if threat, err := sm.detectThreats(ctx, req); err != nil || threat != "" {
			if sm.config.ThreatDetection.LogThreats {
				// Log threat detection
				fmt.Printf("SECURITY THREAT DETECTED: %s for request %s %s\n", threat, req.Method, req.Path)
			}
			
			if sm.config.ThreatDetection.BlockSuspicious {
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
	if sm.config.CSRF != nil && sm.config.CSRF.Enabled {
		if err := sm.validateCSRF(ctx, req); err != nil {
			return &Response{
				ID:         req.ID,
				StatusCode: 403,
				Error:      fmt.Errorf("CSRF validation failed: %w", err),
				Timestamp:  time.Now(),
			}, nil
		}
	}

	// Handle CORS preflight
	if sm.config.CORS != nil && sm.config.CORS.Enabled && req.Method == "OPTIONS" {
		return sm.handleCORSPreflight(ctx, req)
	}

	// Process request through next middleware
	resp, err := next(ctx, req)
	if err != nil {
		return resp, err
	}

	// Add security headers to response
	if resp != nil {
		sm.addSecurityHeaders(resp)
		sm.addCORSHeaders(req, resp)
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

// validateInput validates request input according to configuration
func (sm *SecurityMiddleware) validateInput(ctx context.Context, req *Request) error {
	config := sm.config.InputValidation

	// Validate request size
	if config.MaxRequestSize > 0 {
		// Estimate request size (simplified)
		size := int64(len(req.Method) + len(req.Path))
		for k, v := range req.Headers {
			size += int64(len(k) + len(v))
			if size > config.MaxHeaderSize {
				return fmt.Errorf("header size exceeds limit")
			}
		}
		
		if size > config.MaxRequestSize {
			return fmt.Errorf("request size exceeds limit")
		}
	}

	// Validate content type
	if len(config.AllowedContentTypes) > 0 {
		contentType := req.Headers["Content-Type"]
		if contentType != "" {
			allowed := false
			for _, allowedType := range config.AllowedContentTypes {
				if strings.Contains(contentType, allowedType) {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("content type not allowed: %s", contentType)
			}
		}
	}

	// Validate and sanitize request data
	if req.Body != nil {
		if err := sm.sanitizeData(req, config); err != nil {
			return fmt.Errorf("data sanitization failed: %w", err)
		}
	}

	return nil
}

// sanitizeData sanitizes request data
func (sm *SecurityMiddleware) sanitizeData(req *Request, config *InputValidationConfig) error {
	// HTML sanitization
	if config.SanitizeHTML {
		req.Body = sm.sanitizeHTML(req.Body)
	}

	// Strip control characters
	if config.StripControlChars {
		req.Body = sm.stripControlChars(req.Body)
	}

	return nil
}

// sanitizeHTML sanitizes HTML content
func (sm *SecurityMiddleware) sanitizeHTML(data interface{}) interface{} {
	switch v := data.(type) {
	case string:
		return html.EscapeString(v)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = sm.sanitizeHTML(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = sm.sanitizeHTML(val)
		}
		return result
	default:
		return data
	}
}

// stripControlChars removes control characters from data
func (sm *SecurityMiddleware) stripControlChars(data interface{}) interface{} {
	controlCharsRegex := regexp.MustCompile(`[\x00-\x1f\x7f-\x9f]`)
	
	switch v := data.(type) {
	case string:
		return controlCharsRegex.ReplaceAllString(v, "")
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = sm.stripControlChars(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = sm.stripControlChars(val)
		}
		return result
	default:
		return data
	}
}

// detectThreats detects security threats in the request
func (sm *SecurityMiddleware) detectThreats(ctx context.Context, req *Request) (string, error) {
	config := sm.config.ThreatDetection

	// Check for SQL injection
	if config.SQLInjection {
		if threat := sm.detectSQLInjection(req); threat != "" {
			return threat, nil
		}
	}

	// Check for XSS
	if config.XSSDetection {
		if threat := sm.detectXSS(req); threat != "" {
			return threat, nil
		}
	}

	// Check for path traversal
	if config.PathTraversal {
		if threat := sm.detectPathTraversal(req); threat != "" {
			return threat, nil
		}
	}

	// Check for command injection
	if config.CommandInjection {
		if threat := sm.detectCommandInjection(req); threat != "" {
			return threat, nil
		}
	}

	// Check custom threat patterns
	if threat := sm.detectCustomThreats(req); threat != "" {
		return threat, nil
	}

	return "", nil
}

// detectSQLInjection detects SQL injection attempts
func (sm *SecurityMiddleware) detectSQLInjection(req *Request) string {
	sqlPatterns := []string{
		`(?i)(union\s+select)`,
		`(?i)(select\s+.*\s+from)`,
		`(?i)(insert\s+into)`,
		`(?i)(update\s+.*\s+set)`,
		`(?i)(delete\s+from)`,
		`(?i)(drop\s+table)`,
		`(?i)(or\s+1\s*=\s*1)`,
		`(?i)(and\s+1\s*=\s*1)`,
		`(?i)'.*'`,
		`(?i);\s*--`,
		`(?i)/\*.*\*/`,
	}

	for _, pattern := range sqlPatterns {
		regex := regexp.MustCompile(pattern)
		if sm.checkPatternInRequest(req, regex) {
			return "SQL injection attempt detected"
		}
	}

	return ""
}

// detectXSS detects XSS attempts
func (sm *SecurityMiddleware) detectXSS(req *Request) string {
	xssPatterns := []string{
		`(?i)<script\b[^<]*(?:(?!<\/script>)<[^<]*)*<\/script>`,
		`(?i)<iframe\b[^<]*(?:(?!<\/iframe>)<[^<]*)*<\/iframe>`,
		`(?i)<object\b[^<]*(?:(?!<\/object>)<[^<]*)*<\/object>`,
		`(?i)<embed\b[^>]*>`,
		`(?i)<link\b[^>]*>`,
		`(?i)<meta\b[^>]*>`,
		`(?i)javascript:`,
		`(?i)vbscript:`,
		`(?i)on\w+\s*=`,
		`(?i)expression\s*\(`,
	}

	for _, pattern := range xssPatterns {
		regex := regexp.MustCompile(pattern)
		if sm.checkPatternInRequest(req, regex) {
			return "XSS attempt detected"
		}
	}

	return ""
}

// detectPathTraversal detects path traversal attempts
func (sm *SecurityMiddleware) detectPathTraversal(req *Request) string {
	pathTraversalPatterns := []string{
		`\.\.\/`,
		`\.\.\\`,
		`%2e%2e%2f`,
		`%2e%2e%5c`,
		`..%2f`,
		`..%5c`,
		`%252e%252e%252f`,
	}

	for _, pattern := range pathTraversalPatterns {
		if strings.Contains(strings.ToLower(req.Path), strings.ToLower(pattern)) {
			return "Path traversal attempt detected"
		}
		
		// Check in headers and body
		regex := regexp.MustCompile(pattern)
		if sm.checkPatternInRequest(req, regex) {
			return "Path traversal attempt detected"
		}
	}

	return ""
}

// detectCommandInjection detects command injection attempts
func (sm *SecurityMiddleware) detectCommandInjection(req *Request) string {
	cmdPatterns := []string{
		`(?i);\s*rm\s+`,
		`(?i);\s*cat\s+`,
		`(?i);\s*ls\s+`,
		`(?i);\s*ps\s+`,
		`(?i);\s*id\s*`,
		`(?i);\s*whoami\s*`,
		`(?i);\s*pwd\s*`,
		`(?i)\|\s*nc\s+`,
		`(?i)\|\s*wget\s+`,
		`(?i)\|\s*curl\s+`,
		`(?i)&&\s*rm\s+`,
		`(?i)&&\s*cat\s+`,
	}

	for _, pattern := range cmdPatterns {
		regex := regexp.MustCompile(pattern)
		if sm.checkPatternInRequest(req, regex) {
			return "Command injection attempt detected"
		}
	}

	return ""
}

// detectCustomThreats detects custom threat patterns
func (sm *SecurityMiddleware) detectCustomThreats(req *Request) string {
	for _, regex := range sm.threatRegexs {
		if sm.checkPatternInRequest(req, regex) {
			return "Custom threat pattern detected"
		}
	}

	return ""
}

// checkPatternInRequest checks if a pattern exists in the request
func (sm *SecurityMiddleware) checkPatternInRequest(req *Request, pattern *regexp.Regexp) bool {
	// Check in path
	if pattern.MatchString(req.Path) {
		return true
	}

	// Check in headers
	for k, v := range req.Headers {
		if pattern.MatchString(k) || pattern.MatchString(v) {
			return true
		}
	}

	// Check in body
	if req.Body != nil {
		bodyStr := fmt.Sprintf("%v", req.Body)
		if pattern.MatchString(bodyStr) {
			return true
		}
	}

	return false
}

// validateCSRF validates CSRF tokens
func (sm *SecurityMiddleware) validateCSRF(ctx context.Context, req *Request) error {
	config := sm.config.CSRF

	// Skip safe methods
	safeMethods := config.SafeMethods
	if len(safeMethods) == 0 {
		safeMethods = []string{"GET", "HEAD", "OPTIONS", "TRACE"}
	}

	for _, method := range safeMethods {
		if req.Method == method {
			return nil
		}
	}

	// Skip exempt paths
	for _, path := range config.ExemptPaths {
		if req.Path == path {
			return nil
		}
	}

	// Get CSRF token from request
	token := ""
	if config.TokenHeader != "" {
		token = req.Headers[config.TokenHeader]
	}
	
	if token == "" && config.TokenField != "" {
		// Try to get from body (simplified)
		if bodyMap, ok := req.Body.(map[string]interface{}); ok {
			if tokenVal, ok := bodyMap[config.TokenField].(string); ok {
				token = tokenVal
			}
		}
	}

	if token == "" {
		return fmt.Errorf("CSRF token missing")
	}

	// Validate token (simplified validation)
	sm.mu.RLock()
	tokenTime, exists := sm.csrfTokens[token]
	sm.mu.RUnlock()

	if !exists {
		return fmt.Errorf("invalid CSRF token")
	}

	// Check token expiration (1 hour default)
	if time.Since(tokenTime) > time.Hour {
		sm.mu.Lock()
		delete(sm.csrfTokens, token)
		sm.mu.Unlock()
		return fmt.Errorf("CSRF token expired")
	}

	return nil
}

// handleCORSPreflight handles CORS preflight requests
func (sm *SecurityMiddleware) handleCORSPreflight(ctx context.Context, req *Request) (*Response, error) {
	config := sm.config.CORS
	headers := make(map[string]string)

	// Add CORS headers
	origin := req.Headers["Origin"]
	if sm.isOriginAllowed(origin, config.AllowedOrigins) {
		headers["Access-Control-Allow-Origin"] = origin
	}

	if len(config.AllowedMethods) > 0 {
		headers["Access-Control-Allow-Methods"] = strings.Join(config.AllowedMethods, ", ")
	}

	if len(config.AllowedHeaders) > 0 {
		headers["Access-Control-Allow-Headers"] = strings.Join(config.AllowedHeaders, ", ")
	}

	if config.AllowCredentials {
		headers["Access-Control-Allow-Credentials"] = "true"
	}

	if config.MaxAge > 0 {
		headers["Access-Control-Max-Age"] = fmt.Sprintf("%d", config.MaxAge)
	}

	statusCode := config.OptionsSuccess
	if statusCode == 0 {
		statusCode = 204
	}

	return &Response{
		ID:         req.ID,
		StatusCode: statusCode,
		Headers:    headers,
		Timestamp:  time.Now(),
	}, nil
}

// addCORSHeaders adds CORS headers to response
func (sm *SecurityMiddleware) addCORSHeaders(req *Request, resp *Response) {
	if sm.config.CORS == nil || !sm.config.CORS.Enabled {
		return
	}

	config := sm.config.CORS
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}

	origin := req.Headers["Origin"]
	if sm.isOriginAllowed(origin, config.AllowedOrigins) {
		resp.Headers["Access-Control-Allow-Origin"] = origin
	}

	if len(config.ExposedHeaders) > 0 {
		resp.Headers["Access-Control-Expose-Headers"] = strings.Join(config.ExposedHeaders, ", ")
	}

	if config.AllowCredentials {
		resp.Headers["Access-Control-Allow-Credentials"] = "true"
	}
}

// addSecurityHeaders adds security headers to response
func (sm *SecurityMiddleware) addSecurityHeaders(resp *Response) {
	if sm.config.Headers == nil || !sm.config.Headers.Enabled {
		return
	}

	config := sm.config.Headers
	if resp.Headers == nil {
		resp.Headers = make(map[string]string)
	}

	if config.XFrameOptions != "" {
		resp.Headers["X-Frame-Options"] = config.XFrameOptions
	}

	if config.XContentTypeOptions != "" {
		resp.Headers["X-Content-Type-Options"] = config.XContentTypeOptions
	}

	if config.XXSSProtection != "" {
		resp.Headers["X-XSS-Protection"] = config.XXSSProtection
	}

	if config.ContentSecurityPolicy != "" {
		resp.Headers["Content-Security-Policy"] = config.ContentSecurityPolicy
	}

	if config.StrictTransportSecurity != "" {
		resp.Headers["Strict-Transport-Security"] = config.StrictTransportSecurity
	}

	if config.ReferrerPolicy != "" {
		resp.Headers["Referrer-Policy"] = config.ReferrerPolicy
	}

	if config.PermissionsPolicy != "" {
		resp.Headers["Permissions-Policy"] = config.PermissionsPolicy
	}

	if config.XPermittedCrossDomainPolicies != "" {
		resp.Headers["X-Permitted-Cross-Domain-Policies"] = config.XPermittedCrossDomainPolicies
	}
}

// isOriginAllowed checks if origin is allowed
func (sm *SecurityMiddleware) isOriginAllowed(origin string, allowedOrigins []string) bool {
	if origin == "" {
		return false
	}

	for _, allowed := range allowedOrigins {
		if allowed == "*" {
			return true
		}
		if allowed == origin {
			return true
		}
		// Support wildcard subdomains (simplified)
		if strings.HasPrefix(allowed, "*.") {
			domain := strings.TrimPrefix(allowed, "*.")
			if strings.HasSuffix(origin, domain) {
				return true
			}
		}
	}

	return false
}

// initThreatPatterns initializes threat detection regex patterns
func (sm *SecurityMiddleware) initThreatPatterns() error {
	if sm.config.ThreatDetection == nil || len(sm.config.ThreatDetection.ThreatPatterns) == 0 {
		return nil
	}

	patterns := make([]*regexp.Regexp, 0, len(sm.config.ThreatDetection.ThreatPatterns))
	for _, pattern := range sm.config.ThreatDetection.ThreatPatterns {
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile threat pattern '%s': %w", pattern, err)
		}
		patterns = append(patterns, regex)
	}

	sm.threatRegexs = patterns
	return nil
}

// GenerateCSRFToken generates a new CSRF token
func (sm *SecurityMiddleware) GenerateCSRFToken() string {
	// Generate a simple token (in production, use a cryptographically secure method)
	token := fmt.Sprintf("csrf_%d_%d", time.Now().UnixNano(), time.Now().Unix())
	
	sm.mu.Lock()
	sm.csrfTokens[token] = time.Now()
	sm.mu.Unlock()

	return token
}

// CleanupExpiredTokens removes expired CSRF tokens
func (sm *SecurityMiddleware) CleanupExpiredTokens() {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	now := time.Now()
	for token, tokenTime := range sm.csrfTokens {
		if now.Sub(tokenTime) > time.Hour {
			delete(sm.csrfTokens, token)
		}
	}
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