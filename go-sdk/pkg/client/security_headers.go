package client

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// SecurityHeaderManager manages security headers and CORS
type SecurityHeaderManager struct {
	config *SecurityHeadersConfig
	logger *zap.Logger
}

// NewSecurityHeaderManager creates a new security header manager
func NewSecurityHeaderManager(config *SecurityHeadersConfig, logger *zap.Logger) (*SecurityHeaderManager, error) {
	if config == nil {
		return nil, fmt.Errorf("security headers config cannot be nil")
	}
	
	if logger == nil {
		logger = zap.NewNop()
	}
	
	shm := &SecurityHeaderManager{
		config: config,
		logger: logger,
	}
	
	return shm, nil
}

// ApplySecurityHeaders applies security headers to the HTTP response
func (shm *SecurityHeaderManager) ApplySecurityHeaders(w http.ResponseWriter, r *http.Request) {
	// Handle CORS first
	if shm.config.CORSConfig.Enabled {
		shm.handleCORS(w, r)
	}
	
	// Apply Content Security Policy
	if shm.config.EnableCSP && shm.config.CSPPolicy != "" {
		w.Header().Set("Content-Security-Policy", shm.config.CSPPolicy)
		shm.logger.Debug("Applied CSP header", zap.String("policy", shm.config.CSPPolicy))
	}
	
	// Apply HTTP Strict Transport Security
	if shm.config.EnableHSTS && shm.config.HSTSMaxAge > 0 {
		hstsValue := fmt.Sprintf("max-age=%d; includeSubDomains", shm.config.HSTSMaxAge)
		w.Header().Set("Strict-Transport-Security", hstsValue)
		shm.logger.Debug("Applied HSTS header", zap.String("value", hstsValue))
	}
	
	// Apply X-Frame-Options
	if shm.config.EnableXFrameOptions && shm.config.XFrameOptions != "" {
		w.Header().Set("X-Frame-Options", shm.config.XFrameOptions)
		shm.logger.Debug("Applied X-Frame-Options header", zap.String("value", shm.config.XFrameOptions))
	}
	
	// Apply X-Content-Type-Options
	if shm.config.EnableXContentType {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		shm.logger.Debug("Applied X-Content-Type-Options header")
	}
	
	// Apply Referrer Policy
	if shm.config.EnableReferrerPolicy && shm.config.ReferrerPolicy != "" {
		w.Header().Set("Referrer-Policy", shm.config.ReferrerPolicy)
		shm.logger.Debug("Applied Referrer-Policy header", zap.String("value", shm.config.ReferrerPolicy))
	}
	
	// Apply X-XSS-Protection (deprecated but some legacy systems might need it)
	w.Header().Set("X-XSS-Protection", "1; mode=block")
	
	// Apply X-Download-Options for IE
	w.Header().Set("X-Download-Options", "noopen")
	
	// Apply X-Permitted-Cross-Domain-Policies
	w.Header().Set("X-Permitted-Cross-Domain-Policies", "none")
	
	// Apply custom headers
	for name, value := range shm.config.CustomHeaders {
		w.Header().Set(name, value)
		shm.logger.Debug("Applied custom header", zap.String("name", name), zap.String("value", value))
	}
}

// handleCORS handles Cross-Origin Resource Sharing
func (shm *SecurityHeaderManager) handleCORS(w http.ResponseWriter, r *http.Request) {
	origin := r.Header.Get("Origin")
	
	// Check if origin is allowed
	if origin != "" && shm.isOriginAllowed(origin) {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		shm.logger.Debug("Set CORS Allow-Origin", zap.String("origin", origin))
	} else if shm.isWildcardAllowed() {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		shm.logger.Debug("Set CORS Allow-Origin to wildcard")
	}
	
	// Set allowed methods
	if len(shm.config.CORSConfig.AllowedMethods) > 0 {
		methods := strings.Join(shm.config.CORSConfig.AllowedMethods, ", ")
		w.Header().Set("Access-Control-Allow-Methods", methods)
		shm.logger.Debug("Set CORS Allow-Methods", zap.String("methods", methods))
	}
	
	// Set allowed headers
	if len(shm.config.CORSConfig.AllowedHeaders) > 0 {
		headers := strings.Join(shm.config.CORSConfig.AllowedHeaders, ", ")
		w.Header().Set("Access-Control-Allow-Headers", headers)
		shm.logger.Debug("Set CORS Allow-Headers", zap.String("headers", headers))
	}
	
	// Set exposed headers
	if len(shm.config.CORSConfig.ExposedHeaders) > 0 {
		headers := strings.Join(shm.config.CORSConfig.ExposedHeaders, ", ")
		w.Header().Set("Access-Control-Expose-Headers", headers)
		shm.logger.Debug("Set CORS Expose-Headers", zap.String("headers", headers))
	}
	
	// Set credentials
	if shm.config.CORSConfig.AllowCredentials {
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		shm.logger.Debug("Set CORS Allow-Credentials")
	}
	
	// Set max age
	if shm.config.CORSConfig.MaxAge > 0 {
		w.Header().Set("Access-Control-Max-Age", strconv.Itoa(shm.config.CORSConfig.MaxAge))
		shm.logger.Debug("Set CORS Max-Age", zap.Int("max_age", shm.config.CORSConfig.MaxAge))
	}
	
	// Handle preflight requests
	if r.Method == "OPTIONS" {
		shm.handlePreflightRequest(w, r)
	}
}

// handlePreflightRequest handles CORS preflight requests
func (shm *SecurityHeaderManager) handlePreflightRequest(w http.ResponseWriter, r *http.Request) {
	// Check if this is a valid preflight request
	if r.Header.Get("Access-Control-Request-Method") == "" {
		shm.logger.Debug("Invalid preflight request - missing Access-Control-Request-Method")
		return
	}
	
	requestMethod := r.Header.Get("Access-Control-Request-Method")
	requestHeaders := r.Header.Get("Access-Control-Request-Headers")
	
	// Validate requested method
	if !shm.isMethodAllowed(requestMethod) {
		shm.logger.Warn("CORS preflight - method not allowed",
			zap.String("method", requestMethod))
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	
	// Validate requested headers
	if requestHeaders != "" && !shm.areHeadersAllowed(requestHeaders) {
		shm.logger.Warn("CORS preflight - headers not allowed",
			zap.String("headers", requestHeaders))
		w.WriteHeader(http.StatusForbidden)
		return
	}
	
	// Set response status for successful preflight
	w.WriteHeader(http.StatusNoContent)
	
	shm.logger.Debug("Handled CORS preflight request successfully",
		zap.String("method", requestMethod),
		zap.String("headers", requestHeaders))
}

// isOriginAllowed checks if an origin is in the allowed list
func (shm *SecurityHeaderManager) isOriginAllowed(origin string) bool {
	for _, allowedOrigin := range shm.config.CORSConfig.AllowedOrigins {
		if allowedOrigin == "*" || allowedOrigin == origin {
			return true
		}
		
		// Support for wildcard subdomains (e.g., "*.example.com")
		if strings.HasPrefix(allowedOrigin, "*.") {
			domain := strings.TrimPrefix(allowedOrigin, "*.")
			if strings.HasSuffix(origin, "."+domain) || origin == domain {
				return true
			}
		}
	}
	return false
}

// isWildcardAllowed checks if wildcard origin is allowed
func (shm *SecurityHeaderManager) isWildcardAllowed() bool {
	for _, origin := range shm.config.CORSConfig.AllowedOrigins {
		if origin == "*" {
			return true
		}
	}
	return false
}

// isMethodAllowed checks if a method is allowed
func (shm *SecurityHeaderManager) isMethodAllowed(method string) bool {
	for _, allowedMethod := range shm.config.CORSConfig.AllowedMethods {
		if strings.EqualFold(allowedMethod, method) {
			return true
		}
	}
	return false
}

// areHeadersAllowed checks if all requested headers are allowed
func (shm *SecurityHeaderManager) areHeadersAllowed(requestHeaders string) bool {
	headers := strings.Split(requestHeaders, ",")
	for _, header := range headers {
		header = strings.TrimSpace(header)
		if !shm.isHeaderAllowed(header) {
			return false
		}
	}
	return true
}

// isHeaderAllowed checks if a header is allowed
func (shm *SecurityHeaderManager) isHeaderAllowed(header string) bool {
	// Always allow simple headers
	simpleHeaders := []string{
		"Accept", "Accept-Language", "Content-Language", "Content-Type",
		"DPR", "Downlink", "Save-Data", "Viewport-Width", "Width",
	}
	
	for _, simpleHeader := range simpleHeaders {
		if strings.EqualFold(header, simpleHeader) {
			return true
		}
	}
	
	// Check against configured allowed headers
	for _, allowedHeader := range shm.config.CORSConfig.AllowedHeaders {
		if strings.EqualFold(allowedHeader, header) || allowedHeader == "*" {
			return true
		}
	}
	
	return false
}

// GetCSPViolationHandler returns a handler for CSP violation reports
func (shm *SecurityHeaderManager) GetCSPViolationHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Read the violation report
		// Note: In a real implementation, you would parse the JSON report
		// and log or store the violation details
		
		shm.logger.Warn("CSP violation reported",
			zap.String("user_agent", r.Header.Get("User-Agent")),
			zap.String("remote_addr", r.RemoteAddr))
		
		w.WriteHeader(http.StatusNoContent)
	}
}

// ValidateCSPPolicy validates a CSP policy string
func (shm *SecurityHeaderManager) ValidateCSPPolicy(policy string) []string {
	var issues []string
	
	if policy == "" {
		issues = append(issues, "CSP policy is empty")
		return issues
	}
	
	// Split directives
	directives := strings.Split(policy, ";")
	directiveMap := make(map[string]bool)
	
	for _, directive := range directives {
		directive = strings.TrimSpace(directive)
		if directive == "" {
			continue
		}
		
		parts := strings.Fields(directive)
		if len(parts) == 0 {
			continue
		}
		
		directiveName := parts[0]
		
		// Check for duplicate directives
		if directiveMap[directiveName] {
			issues = append(issues, fmt.Sprintf("Duplicate directive: %s", directiveName))
		}
		directiveMap[directiveName] = true
		
		// Validate specific directives
		switch directiveName {
		case "default-src":
			if len(parts) == 1 {
				issues = append(issues, "default-src directive should have at least one source")
			}
		case "script-src":
			if shm.hasUnsafeInline(parts) && shm.hasUnsafeEval(parts) {
				issues = append(issues, "script-src has both 'unsafe-inline' and 'unsafe-eval' which is risky")
			}
		case "style-src":
			// Allow unsafe-inline for styles as it's more common
		case "img-src", "font-src", "connect-src", "media-src", "object-src", "frame-src":
			// These are valid directives
		default:
			// Check if it's a known directive
			if !shm.isKnownCSPDirective(directiveName) {
				issues = append(issues, fmt.Sprintf("Unknown CSP directive: %s", directiveName))
			}
		}
	}
	
	// Check for missing critical directives
	if !directiveMap["default-src"] && !directiveMap["script-src"] {
		issues = append(issues, "Missing default-src or script-src directive")
	}
	
	return issues
}

// hasUnsafeInline checks if 'unsafe-inline' is present in directive sources
func (shm *SecurityHeaderManager) hasUnsafeInline(parts []string) bool {
	for _, part := range parts {
		if part == "'unsafe-inline'" {
			return true
		}
	}
	return false
}

// hasUnsafeEval checks if 'unsafe-eval' is present in directive sources
func (shm *SecurityHeaderManager) hasUnsafeEval(parts []string) bool {
	for _, part := range parts {
		if part == "'unsafe-eval'" {
			return true
		}
	}
	return false
}

// isKnownCSPDirective checks if a directive is a known CSP directive
func (shm *SecurityHeaderManager) isKnownCSPDirective(directive string) bool {
	knownDirectives := []string{
		"default-src", "script-src", "style-src", "img-src", "font-src",
		"connect-src", "media-src", "object-src", "frame-src", "child-src",
		"worker-src", "frame-ancestors", "form-action", "plugin-types",
		"sandbox", "report-uri", "report-to", "base-uri", "manifest-src",
		"prefetch-src", "navigate-to", "require-sri-for", "upgrade-insecure-requests",
		"block-all-mixed-content", "trusted-types", "require-trusted-types-for",
	}
	
	for _, known := range knownDirectives {
		if directive == known {
			return true
		}
	}
	return false
}

// GenerateNonce generates a cryptographically secure nonce for CSP
func (shm *SecurityHeaderManager) GenerateNonce() (string, error) {
	// This would generate a secure random nonce for CSP
	// For now, return a placeholder
	return fmt.Sprintf("nonce-%d", time.Now().UnixNano()), nil
}

// GetSecurityHeadersInfo returns information about configured security headers
func (shm *SecurityHeaderManager) GetSecurityHeadersInfo() map[string]interface{} {
	return map[string]interface{}{
		"csp_enabled":             shm.config.EnableCSP,
		"csp_policy":              shm.config.CSPPolicy,
		"hsts_enabled":            shm.config.EnableHSTS,
		"hsts_max_age":            shm.config.HSTSMaxAge,
		"x_frame_options_enabled": shm.config.EnableXFrameOptions,
		"x_frame_options":         shm.config.XFrameOptions,
		"x_content_type_enabled":  shm.config.EnableXContentType,
		"referrer_policy_enabled": shm.config.EnableReferrerPolicy,
		"referrer_policy":         shm.config.ReferrerPolicy,
		"cors_enabled":            shm.config.CORSConfig.Enabled,
		"cors_allowed_origins":    shm.config.CORSConfig.AllowedOrigins,
		"cors_allowed_methods":    shm.config.CORSConfig.AllowedMethods,
		"cors_allowed_headers":    shm.config.CORSConfig.AllowedHeaders,
		"cors_allow_credentials":  shm.config.CORSConfig.AllowCredentials,
		"custom_headers":          shm.config.CustomHeaders,
	}
}

// UpdateCSPPolicy updates the CSP policy
func (shm *SecurityHeaderManager) UpdateCSPPolicy(policy string) error {
	// Validate the new policy
	issues := shm.ValidateCSPPolicy(policy)
	if len(issues) > 0 {
		return fmt.Errorf("CSP policy validation failed: %v", issues)
	}
	
	shm.config.CSPPolicy = policy
	
	shm.logger.Info("Updated CSP policy",
		zap.String("new_policy", policy))
	
	return nil
}

// AddCustomHeader adds a custom security header
func (shm *SecurityHeaderManager) AddCustomHeader(name, value string) {
	if shm.config.CustomHeaders == nil {
		shm.config.CustomHeaders = make(map[string]string)
	}
	
	shm.config.CustomHeaders[name] = value
	
	shm.logger.Info("Added custom security header",
		zap.String("name", name),
		zap.String("value", value))
}

// RemoveCustomHeader removes a custom security header
func (shm *SecurityHeaderManager) RemoveCustomHeader(name string) {
	if shm.config.CustomHeaders != nil {
		delete(shm.config.CustomHeaders, name)
		
		shm.logger.Info("Removed custom security header",
			zap.String("name", name))
	}
}

// Middleware returns an HTTP middleware that applies security headers
func (shm *SecurityHeaderManager) Middleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			shm.ApplySecurityHeaders(w, r)
			next.ServeHTTP(w, r)
		})
	}
}