package middleware

import (
	"context"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
)

// SecureMiddlewareAdapter adapts secure middleware to the standard interface
type SecureMiddlewareAdapter struct {
	middleware interface{}
	name       string
	priority   int
	enabled    bool
}

// NewSecureMiddlewareAdapter creates a new adapter for secure middleware
func NewSecureMiddlewareAdapter(middleware interface{}) Middleware {
	adapter := &SecureMiddlewareAdapter{
		middleware: middleware,
		enabled:    true,
	}

	// Set adapter properties based on middleware type
	switch m := middleware.(type) {
	case *security.SecureJWTMiddleware:
		adapter.name = m.Name()
		adapter.priority = m.Priority()
		adapter.enabled = m.Enabled()
	case *security.SecureOAuth2Middleware:
		adapter.name = m.Name()
		adapter.priority = m.Priority()
		adapter.enabled = m.Enabled()
	case *security.SecureAPIKeyMiddleware:
		adapter.name = m.Name()
		adapter.priority = m.Priority()
		adapter.enabled = m.Enabled()
	default:
		adapter.name = "secure_middleware"
		adapter.priority = 100
	}

	return adapter
}

// Name returns the middleware name
func (sma *SecureMiddlewareAdapter) Name() string {
	return sma.name
}

// Process processes the request through the secure middleware
func (sma *SecureMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert types for secure middleware
	secureReq := &security.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   req.Headers,
		Body:      req.Body,
		Metadata:  req.Metadata,
		Timestamp: req.Timestamp,
	}

	secureNext := func(ctx context.Context, secReq *security.Request) (*security.Response, error) {
		// Convert back to standard middleware types
		standardReq := &Request{
			ID:        secReq.ID,
			Method:    secReq.Method,
			Path:      secReq.Path,
			Headers:   secReq.Headers,
			Body:      secReq.Body,
			Metadata:  secReq.Metadata,
			Timestamp: secReq.Timestamp,
		}

		standardResp, err := next(ctx, standardReq)
		if err != nil {
			return nil, err
		}

		// Convert response back to secure type
		if standardResp == nil {
			return nil, nil
		}

		return &security.Response{
			ID:         standardResp.ID,
			StatusCode: standardResp.StatusCode,
			Headers:    standardResp.Headers,
			Body:       standardResp.Body,
			Error:      standardResp.Error,
			Metadata:   standardResp.Metadata,
			Timestamp:  standardResp.Timestamp,
			Duration:   standardResp.Duration,
		}, nil
	}

	// Process through the appropriate secure middleware
	var secureResp *security.Response
	var err error

	switch m := sma.middleware.(type) {
	case *security.SecureJWTMiddleware:
		secureResp, err = m.Process(ctx, secureReq, secureNext)
	case *security.SecureOAuth2Middleware:
		secureResp, err = m.Process(ctx, secureReq, secureNext)
	case *security.SecureAPIKeyMiddleware:
		secureResp, err = m.Process(ctx, secureReq, secureNext)
	default:
		// Fallback to next handler if middleware type is unknown
		return next(ctx, req)
	}

	if err != nil {
		return nil, err
	}

	if secureResp == nil {
		return nil, nil
	}

	// Convert response back to standard type
	return &Response{
		ID:         secureResp.ID,
		StatusCode: secureResp.StatusCode,
		Headers:    secureResp.Headers,
		Body:       secureResp.Body,
		Error:      secureResp.Error,
		Metadata:   secureResp.Metadata,
		Timestamp:  secureResp.Timestamp,
		Duration:   secureResp.Duration,
	}, nil
}

// Configure configures the secure middleware
func (sma *SecureMiddlewareAdapter) Configure(config map[string]interface{}) error {
	if enabled, ok := config["enabled"].(bool); ok {
		sma.enabled = enabled
	}

	if priority, ok := config["priority"].(int); ok {
		sma.priority = priority
	}

	// Forward configuration to the underlying middleware
	switch m := sma.middleware.(type) {
	case *security.SecureJWTMiddleware:
		return m.Configure(config)
	case *security.SecureOAuth2Middleware:
		return m.Configure(config)
	case *security.SecureAPIKeyMiddleware:
		return m.Configure(config)
	}

	return nil
}

// Enabled returns whether the middleware is enabled
func (sma *SecureMiddlewareAdapter) Enabled() bool {
	return sma.enabled
}

// Priority returns the middleware priority
func (sma *SecureMiddlewareAdapter) Priority() int {
	return sma.priority
}

// GetSecurityMetrics returns security metrics if available
func (sma *SecureMiddlewareAdapter) GetSecurityMetrics() map[string]interface{} {
	switch m := sma.middleware.(type) {
	case *security.SecureJWTMiddleware:
		return map[string]interface{}{
			"middleware_type": "secure_jwt",
			"enabled":         m.Enabled(),
		}
	case *security.SecureOAuth2Middleware:
		return map[string]interface{}{
			"middleware_type": "secure_oauth2",
			"enabled":         m.Enabled(),
		}
	case *security.SecureAPIKeyMiddleware:
		return map[string]interface{}{
			"middleware_type": "secure_apikey",
			"enabled":         m.Enabled(),
		}
	default:
		return map[string]interface{}{
			"middleware_type": "unknown_secure",
			"security_level":  "enhanced",
			"timestamp":       time.Now(),
		}
	}
}

// IsSecureMiddleware returns true if this is a secure middleware adapter
func (sma *SecureMiddlewareAdapter) IsSecureMiddleware() bool {
	return true
}

// GetUnderlyingMiddleware returns the underlying secure middleware
func (sma *SecureMiddlewareAdapter) GetUnderlyingMiddleware() interface{} {
	return sma.middleware
}
