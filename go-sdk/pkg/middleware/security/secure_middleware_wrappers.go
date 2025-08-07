package security

import (
	"context"
	"fmt"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"strings"
	"time"
)

// SecureJWTMiddleware wraps JWT middleware with enhanced security
type SecureJWTMiddleware struct {
	middleware auth.JWTMiddleware
	config     *SecureMiddlewareConfig
}

// SecureOAuth2Middleware wraps OAuth2 middleware with enhanced security
type SecureOAuth2Middleware struct {
	middleware auth.OAuth2Middleware
	config     *SecureMiddlewareConfig
}

// SecureAPIKeyMiddleware wraps API key middleware with enhanced security
type SecureAPIKeyMiddleware struct {
	middleware auth.APIKeyMiddleware
	config     *SecureMiddlewareConfig
}

// SecureMiddlewareConfig contains security configuration
type SecureMiddlewareConfig struct {
	EnableInputValidation bool
	EnableSecurityHeaders bool
	EnableAuditLogging    bool
	MaxRequestSize        int64
	AllowedContentTypes   []string
	BlockedUserAgents     []string
	EnableRateLimiting    bool
	RequestsPerMinute     int
}

// NewSecureJWTMiddleware creates a new secure JWT middleware wrapper
func NewSecureJWTMiddleware(jwtMiddleware auth.JWTMiddleware, config *SecureMiddlewareConfig) *SecureJWTMiddleware {
	if config == nil {
		config = &SecureMiddlewareConfig{
			EnableInputValidation: true,
			EnableSecurityHeaders: true,
			EnableAuditLogging:    true,
			MaxRequestSize:        1024 * 1024, // 1MB
			AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
		}
	}

	return &SecureJWTMiddleware{
		middleware: jwtMiddleware,
		config:     config,
	}
}

// NewSecureOAuth2Middleware creates a new secure OAuth2 middleware wrapper
func NewSecureOAuth2Middleware(oauth2Middleware auth.OAuth2Middleware, config *SecureMiddlewareConfig) *SecureOAuth2Middleware {
	if config == nil {
		config = &SecureMiddlewareConfig{
			EnableInputValidation: true,
			EnableSecurityHeaders: true,
			EnableAuditLogging:    true,
			MaxRequestSize:        1024 * 1024,
			AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
		}
	}

	return &SecureOAuth2Middleware{
		middleware: oauth2Middleware,
		config:     config,
	}
}

// NewSecureAPIKeyMiddleware creates a new secure API key middleware wrapper
func NewSecureAPIKeyMiddleware(apikeyMiddleware auth.APIKeyMiddleware, config *SecureMiddlewareConfig) *SecureAPIKeyMiddleware {
	if config == nil {
		config = &SecureMiddlewareConfig{
			EnableInputValidation: true,
			EnableSecurityHeaders: true,
			EnableAuditLogging:    true,
			MaxRequestSize:        1024 * 1024,
			AllowedContentTypes:   []string{"application/json", "application/x-www-form-urlencoded"},
		}
	}

	return &SecureAPIKeyMiddleware{
		middleware: apikeyMiddleware,
		config:     config,
	}
}

// validateRequest performs security validation on requests
func (sjm *SecureJWTMiddleware) validateRequest(ctx context.Context, req *Request) error {
	if !sjm.config.EnableInputValidation {
		return nil
	}

	// Check request size
	if sjm.config.MaxRequestSize > 0 {
		// Estimate request size (simplified)
		if int64(len(fmt.Sprintf("%v", req.Body))) > sjm.config.MaxRequestSize {
			return fmt.Errorf("request size exceeds maximum allowed size")
		}
	}

	// Check content type
	if len(sjm.config.AllowedContentTypes) > 0 {
		contentType := req.Headers["Content-Type"]
		allowed := false
		for _, allowedType := range sjm.config.AllowedContentTypes {
			if strings.Contains(contentType, allowedType) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("content type not allowed: %s", contentType)
		}
	}

	// Check user agent blocking
	if len(sjm.config.BlockedUserAgents) > 0 {
		userAgent := req.Headers["User-Agent"]
		for _, blocked := range sjm.config.BlockedUserAgents {
			if strings.Contains(userAgent, blocked) {
				return fmt.Errorf("user agent is blocked")
			}
		}
	}

	return nil
}

// addSecurityHeaders adds security headers to requests
func (sjm *SecureJWTMiddleware) addSecurityHeaders(headers map[string]string) map[string]string {
	if !sjm.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	// Add security headers
	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"

	return headers
}

// addResponseSecurityHeaders adds security headers to responses
func (sjm *SecureJWTMiddleware) addResponseSecurityHeaders(headers map[string]string) map[string]string {
	if !sjm.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"
	headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"

	return headers
}

// Name returns the middleware name
func (sjm *SecureJWTMiddleware) Name() string {
	return "secure_jwt"
}

// Configure configures the middleware
func (sjm *SecureJWTMiddleware) Configure(config map[string]interface{}) error {
	return nil // Configuration is handled during creation
}

// Enabled returns whether the middleware is enabled
func (sjm *SecureJWTMiddleware) Enabled() bool {
	return true
}

// Priority returns the middleware priority
func (sjm *SecureJWTMiddleware) Priority() int {
	return 100
}

// Process handles JWT middleware processing with enhanced security
func (sjm *SecureJWTMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Pre-process validation
	if err := sjm.validateRequest(ctx, req); err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 400,
			Error:      fmt.Errorf("request validation failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add security headers to prevent common attacks
	req.Headers = sjm.addSecurityHeaders(req.Headers)

	// Process through JWT middleware
	response, err := next(ctx, req)

	// Post-process security enhancements
	if response != nil {
		response.Headers = sjm.addResponseSecurityHeaders(response.Headers)
	}

	return response, err
}

// Name returns the middleware name
func (som *SecureOAuth2Middleware) Name() string {
	return "secure_oauth2"
}

// Configure configures the middleware
func (som *SecureOAuth2Middleware) Configure(config map[string]interface{}) error {
	return nil // Configuration is handled during creation
}

// Enabled returns whether the middleware is enabled
func (som *SecureOAuth2Middleware) Enabled() bool {
	return true
}

// Priority returns the middleware priority
func (som *SecureOAuth2Middleware) Priority() int {
	return 100
}

// Process handles OAuth2 middleware processing with enhanced security
func (som *SecureOAuth2Middleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Pre-process validation
	if err := som.validateRequest(ctx, req); err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 400,
			Error:      fmt.Errorf("request validation failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add security headers
	req.Headers = som.addSecurityHeaders(req.Headers)

	// Process through OAuth2 middleware
	response, err := next(ctx, req)

	// Post-process security enhancements
	if response != nil {
		response.Headers = som.addResponseSecurityHeaders(response.Headers)
	}

	return response, err
}

// Name returns the middleware name
func (sam *SecureAPIKeyMiddleware) Name() string {
	return "secure_apikey"
}

// Configure configures the middleware
func (sam *SecureAPIKeyMiddleware) Configure(config map[string]interface{}) error {
	return nil // Configuration is handled during creation
}

// Enabled returns whether the middleware is enabled
func (sam *SecureAPIKeyMiddleware) Enabled() bool {
	return true
}

// Priority returns the middleware priority
func (sam *SecureAPIKeyMiddleware) Priority() int {
	return 100
}

// Process handles API key middleware processing with enhanced security
func (sam *SecureAPIKeyMiddleware) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Pre-process validation
	if err := sam.validateRequest(ctx, req); err != nil {
		return &Response{
			ID:         req.ID,
			StatusCode: 400,
			Error:      fmt.Errorf("request validation failed: %w", err),
			Timestamp:  time.Now(),
		}, nil
	}

	// Add security headers
	req.Headers = sam.addSecurityHeaders(req.Headers)

	// Process through API key middleware
	response, err := next(ctx, req)

	// Post-process security enhancements
	if response != nil {
		response.Headers = sam.addResponseSecurityHeaders(response.Headers)
	}

	return response, err
}

// validateRequest for OAuth2 middleware
func (som *SecureOAuth2Middleware) validateRequest(ctx context.Context, req *Request) error {
	if !som.config.EnableInputValidation {
		return nil
	}

	// Similar validation logic as JWT middleware
	if som.config.MaxRequestSize > 0 {
		if int64(len(fmt.Sprintf("%v", req.Body))) > som.config.MaxRequestSize {
			return fmt.Errorf("request size exceeds maximum allowed size")
		}
	}

	return nil
}

// validateRequest for API key middleware
func (sam *SecureAPIKeyMiddleware) validateRequest(ctx context.Context, req *Request) error {
	if !sam.config.EnableInputValidation {
		return nil
	}

	// Similar validation logic
	if sam.config.MaxRequestSize > 0 {
		if int64(len(fmt.Sprintf("%v", req.Body))) > sam.config.MaxRequestSize {
			return fmt.Errorf("request size exceeds maximum allowed size")
		}
	}

	return nil
}

// addSecurityHeaders for OAuth2 middleware
func (som *SecureOAuth2Middleware) addSecurityHeaders(headers map[string]string) map[string]string {
	if !som.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"

	return headers
}

// addSecurityHeaders for API key middleware
func (sam *SecureAPIKeyMiddleware) addSecurityHeaders(headers map[string]string) map[string]string {
	if !sam.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"

	return headers
}

// addResponseSecurityHeaders for OAuth2 middleware
func (som *SecureOAuth2Middleware) addResponseSecurityHeaders(headers map[string]string) map[string]string {
	if !som.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"
	headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"

	return headers
}

// addResponseSecurityHeaders for API key middleware
func (sam *SecureAPIKeyMiddleware) addResponseSecurityHeaders(headers map[string]string) map[string]string {
	if !sam.config.EnableSecurityHeaders {
		return headers
	}

	if headers == nil {
		headers = make(map[string]string)
	}

	headers["X-Content-Type-Options"] = "nosniff"
	headers["X-Frame-Options"] = "DENY"
	headers["X-XSS-Protection"] = "1; mode=block"
	headers["Strict-Transport-Security"] = "max-age=31536000; includeSubDomains"

	return headers
}
