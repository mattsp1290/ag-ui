package middleware

import (
	"context"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/observability"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/ratelimit"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/transform"
)

// Type conversion functions for auth package

// ConvertToAuthRequest converts main Request to auth.Request
func ConvertToAuthRequest(req *Request) *auth.Request {
	if req == nil {
		return nil
	}
	return &auth.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertFromAuthRequest converts auth.Request to main Request
func ConvertFromAuthRequest(req *auth.Request) *Request {
	if req == nil {
		return nil
	}
	return &Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertToAuthResponse converts main Response to auth.Response
func ConvertToAuthResponse(resp *Response) *auth.Response {
	if resp == nil {
		return nil
	}
	return &auth.Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// ConvertFromAuthResponse converts auth.Response to main Response
func ConvertFromAuthResponse(resp *auth.Response) *Response {
	if resp == nil {
		return nil
	}
	return &Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// Type conversion functions for observability package

// ConvertToObservabilityRequest converts main Request to observability.Request
func ConvertToObservabilityRequest(req *Request) *observability.Request {
	if req == nil {
		return nil
	}
	return &observability.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertFromObservabilityResponse converts observability.Response to main Response
func ConvertFromObservabilityResponse(resp *observability.Response) *Response {
	if resp == nil {
		return nil
	}
	return &Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// Type conversion functions for ratelimit package

// ConvertToRateLimitRequest converts main Request to ratelimit.Request
func ConvertToRateLimitRequest(req *Request) *ratelimit.Request {
	if req == nil {
		return nil
	}
	return &ratelimit.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertFromRateLimitResponse converts ratelimit.Response to main Response
func ConvertFromRateLimitResponse(resp *ratelimit.Response) *Response {
	if resp == nil {
		return nil
	}
	return &Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// Type conversion functions for security package

// ConvertToSecurityRequest converts main Request to security.Request
func ConvertToSecurityRequest(req *Request) *security.Request {
	if req == nil {
		return nil
	}
	return &security.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertFromSecurityResponse converts security.Response to main Response
func ConvertFromSecurityResponse(resp *security.Response) *Response {
	if resp == nil {
		return nil
	}
	return &Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// Type conversion functions for transform package

// ConvertToTransformRequest converts main Request to transform.Request
func ConvertToTransformRequest(req *Request) *transform.Request {
	if req == nil {
		return nil
	}
	return &transform.Request{
		ID:        req.ID,
		Method:    req.Method,
		Path:      req.Path,
		Headers:   copyStringMap(req.Headers),
		Body:      req.Body,
		Metadata:  copyInterfaceMap(req.Metadata),
		Timestamp: req.Timestamp,
	}
}

// ConvertFromTransformResponse converts transform.Response to main Response
func ConvertFromTransformResponse(resp *transform.Response) *Response {
	if resp == nil {
		return nil
	}
	return &Response{
		ID:         resp.ID,
		StatusCode: resp.StatusCode,
		Headers:    copyStringMap(resp.Headers),
		Body:       resp.Body,
		Error:      resp.Error,
		Metadata:   copyInterfaceMap(resp.Metadata),
		Timestamp:  resp.Timestamp,
		Duration:   resp.Duration,
	}
}

// Utility functions for deep copying maps
func copyStringMap(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	dst := make(map[string]string, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyInterfaceMap(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	dst := make(map[string]interface{}, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Wrapper middleware interfaces

// AuthMiddleware interface for auth package middleware
type AuthMiddleware interface {
	Name() string
	Process(ctx context.Context, req *auth.Request, next auth.NextHandler) (*auth.Response, error)
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
}

// ObservabilityMiddleware interface for observability package middleware
type ObservabilityMiddleware interface {
	Name() string
	Process(ctx context.Context, req *observability.Request, next observability.NextHandler) (*observability.Response, error)
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
}

// RateLimitMiddleware interface for ratelimit package middleware
type RateLimitMiddleware interface {
	Name() string
	Process(ctx context.Context, req *ratelimit.Request, next ratelimit.NextHandler) (*ratelimit.Response, error)
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
}

// SecurityMiddleware interface for security package middleware
type SecurityMiddleware interface {
	Name() string
	Process(ctx context.Context, req *security.Request, next security.NextHandler) (*security.Response, error)
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
}

// TransformMiddleware interface for transform package middleware
type TransformMiddleware interface {
	Name() string
	Process(ctx context.Context, req *transform.Request, next transform.NextHandler) (*transform.Response, error)
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
}

// Adapter middleware structs

// AuthMiddlewareAdapter wraps auth middleware to implement main Middleware interface
type AuthMiddlewareAdapter struct {
	authMiddleware AuthMiddleware
}

// NewAuthMiddlewareAdapter creates a new auth middleware adapter
func NewAuthMiddlewareAdapter(authMiddleware AuthMiddleware) *AuthMiddlewareAdapter {
	return &AuthMiddlewareAdapter{
		authMiddleware: authMiddleware,
	}
}

func (a *AuthMiddlewareAdapter) Name() string {
	return a.authMiddleware.Name()
}

func (a *AuthMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert main types to auth types
	authReq := ConvertToAuthRequest(req)

	// Create auth NextHandler wrapper
	authNext := func(ctx context.Context, authReq *auth.Request) (*auth.Response, error) {
		mainReq := ConvertFromAuthRequest(authReq)
		mainResp, err := next(ctx, mainReq)
		return ConvertToAuthResponse(mainResp), err
	}

	// Call auth middleware
	authResp, err := a.authMiddleware.Process(ctx, authReq, authNext)

	// Convert auth response back to main response
	return ConvertFromAuthResponse(authResp), err
}

func (a *AuthMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return a.authMiddleware.Configure(config)
}

func (a *AuthMiddlewareAdapter) Enabled() bool {
	return a.authMiddleware.Enabled()
}

func (a *AuthMiddlewareAdapter) Priority() int {
	return a.authMiddleware.Priority()
}

// ObservabilityMiddlewareAdapter wraps observability middleware to implement main Middleware interface
type ObservabilityMiddlewareAdapter struct {
	observabilityMiddleware ObservabilityMiddleware
}

// NewObservabilityMiddlewareAdapter creates a new observability middleware adapter
func NewObservabilityMiddlewareAdapter(observabilityMiddleware ObservabilityMiddleware) *ObservabilityMiddlewareAdapter {
	return &ObservabilityMiddlewareAdapter{
		observabilityMiddleware: observabilityMiddleware,
	}
}

func (o *ObservabilityMiddlewareAdapter) Name() string {
	return o.observabilityMiddleware.Name()
}

func (o *ObservabilityMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert main types to observability types
	obsReq := ConvertToObservabilityRequest(req)

	// Create observability NextHandler wrapper
	obsNext := func(ctx context.Context, obsReq *observability.Request) (*observability.Response, error) {
		// Convert observability request to main request
		mainReq := &Request{
			ID:        obsReq.ID,
			Method:    obsReq.Method,
			Path:      obsReq.Path,
			Headers:   copyStringMap(obsReq.Headers),
			Body:      obsReq.Body,
			Metadata:  copyInterfaceMap(obsReq.Metadata),
			Timestamp: obsReq.Timestamp,
		}
		mainResp, err := next(ctx, mainReq)
		if err != nil || mainResp == nil {
			return nil, err
		}
		// Convert main response to observability response
		return &observability.Response{
			ID:         mainResp.ID,
			StatusCode: mainResp.StatusCode,
			Headers:    copyStringMap(mainResp.Headers),
			Body:       mainResp.Body,
			Error:      mainResp.Error,
			Metadata:   copyInterfaceMap(mainResp.Metadata),
			Timestamp:  mainResp.Timestamp,
			Duration:   mainResp.Duration,
		}, nil
	}

	// Call observability middleware
	obsResp, err := o.observabilityMiddleware.Process(ctx, obsReq, obsNext)

	// Convert observability response back to main response
	return ConvertFromObservabilityResponse(obsResp), err
}

func (o *ObservabilityMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return o.observabilityMiddleware.Configure(config)
}

func (o *ObservabilityMiddlewareAdapter) Enabled() bool {
	return o.observabilityMiddleware.Enabled()
}

func (o *ObservabilityMiddlewareAdapter) Priority() int {
	return o.observabilityMiddleware.Priority()
}

// RateLimitMiddlewareAdapter wraps rate limit middleware to implement main Middleware interface
type RateLimitMiddlewareAdapter struct {
	rateLimitMiddleware RateLimitMiddleware
}

// NewRateLimitMiddlewareAdapter creates a new rate limit middleware adapter
func NewRateLimitMiddlewareAdapter(rateLimitMiddleware RateLimitMiddleware) *RateLimitMiddlewareAdapter {
	return &RateLimitMiddlewareAdapter{
		rateLimitMiddleware: rateLimitMiddleware,
	}
}

func (r *RateLimitMiddlewareAdapter) Name() string {
	return r.rateLimitMiddleware.Name()
}

func (r *RateLimitMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert main types to ratelimit types
	rlReq := ConvertToRateLimitRequest(req)

	// Create ratelimit NextHandler wrapper
	rlNext := func(ctx context.Context, rlReq *ratelimit.Request) (*ratelimit.Response, error) {
		// Convert ratelimit request to main request
		mainReq := &Request{
			ID:        rlReq.ID,
			Method:    rlReq.Method,
			Path:      rlReq.Path,
			Headers:   copyStringMap(rlReq.Headers),
			Body:      rlReq.Body,
			Metadata:  copyInterfaceMap(rlReq.Metadata),
			Timestamp: rlReq.Timestamp,
		}
		mainResp, err := next(ctx, mainReq)
		if err != nil || mainResp == nil {
			return nil, err
		}
		// Convert main response to ratelimit response
		return &ratelimit.Response{
			ID:         mainResp.ID,
			StatusCode: mainResp.StatusCode,
			Headers:    copyStringMap(mainResp.Headers),
			Body:       mainResp.Body,
			Error:      mainResp.Error,
			Metadata:   copyInterfaceMap(mainResp.Metadata),
			Timestamp:  mainResp.Timestamp,
			Duration:   mainResp.Duration,
		}, nil
	}

	// Call ratelimit middleware
	rlResp, err := r.rateLimitMiddleware.Process(ctx, rlReq, rlNext)

	// Convert ratelimit response back to main response
	return ConvertFromRateLimitResponse(rlResp), err
}

func (r *RateLimitMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return r.rateLimitMiddleware.Configure(config)
}

func (r *RateLimitMiddlewareAdapter) Enabled() bool {
	return r.rateLimitMiddleware.Enabled()
}

func (r *RateLimitMiddlewareAdapter) Priority() int {
	return r.rateLimitMiddleware.Priority()
}

// SecurityMiddlewareAdapter wraps security middleware to implement main Middleware interface
type SecurityMiddlewareAdapter struct {
	securityMiddleware SecurityMiddleware
}

// NewSecurityMiddlewareAdapter creates a new security middleware adapter
func NewSecurityMiddlewareAdapter(securityMiddleware SecurityMiddleware) *SecurityMiddlewareAdapter {
	return &SecurityMiddlewareAdapter{
		securityMiddleware: securityMiddleware,
	}
}

func (s *SecurityMiddlewareAdapter) Name() string {
	return s.securityMiddleware.Name()
}

func (s *SecurityMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert main types to security types
	secReq := ConvertToSecurityRequest(req)

	// Create security NextHandler wrapper
	secNext := func(ctx context.Context, secReq *security.Request) (*security.Response, error) {
		// Convert security request to main request
		mainReq := &Request{
			ID:        secReq.ID,
			Method:    secReq.Method,
			Path:      secReq.Path,
			Headers:   copyStringMap(secReq.Headers),
			Body:      secReq.Body,
			Metadata:  copyInterfaceMap(secReq.Metadata),
			Timestamp: secReq.Timestamp,
		}
		mainResp, err := next(ctx, mainReq)
		if err != nil || mainResp == nil {
			return nil, err
		}
		// Convert main response to security response
		return &security.Response{
			ID:         mainResp.ID,
			StatusCode: mainResp.StatusCode,
			Headers:    copyStringMap(mainResp.Headers),
			Body:       mainResp.Body,
			Error:      mainResp.Error,
			Metadata:   copyInterfaceMap(mainResp.Metadata),
			Timestamp:  mainResp.Timestamp,
			Duration:   mainResp.Duration,
		}, nil
	}

	// Call security middleware
	secResp, err := s.securityMiddleware.Process(ctx, secReq, secNext)

	// Convert security response back to main response
	return ConvertFromSecurityResponse(secResp), err
}

func (s *SecurityMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return s.securityMiddleware.Configure(config)
}

func (s *SecurityMiddlewareAdapter) Enabled() bool {
	return s.securityMiddleware.Enabled()
}

func (s *SecurityMiddlewareAdapter) Priority() int {
	return s.securityMiddleware.Priority()
}

// TransformMiddlewareAdapter wraps transform middleware to implement main Middleware interface
type TransformMiddlewareAdapter struct {
	transformMiddleware TransformMiddleware
}

// NewTransformMiddlewareAdapter creates a new transform middleware adapter
func NewTransformMiddlewareAdapter(transformMiddleware TransformMiddleware) *TransformMiddlewareAdapter {
	return &TransformMiddlewareAdapter{
		transformMiddleware: transformMiddleware,
	}
}

func (t *TransformMiddlewareAdapter) Name() string {
	return t.transformMiddleware.Name()
}

func (t *TransformMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert main types to transform types
	transformReq := ConvertToTransformRequest(req)

	// Create transform NextHandler wrapper
	transformNext := func(ctx context.Context, transformReq *transform.Request) (*transform.Response, error) {
		// Convert transform request to main request
		mainReq := &Request{
			ID:        transformReq.ID,
			Method:    transformReq.Method,
			Path:      transformReq.Path,
			Headers:   copyStringMap(transformReq.Headers),
			Body:      transformReq.Body,
			Metadata:  copyInterfaceMap(transformReq.Metadata),
			Timestamp: transformReq.Timestamp,
		}
		mainResp, err := next(ctx, mainReq)
		if err != nil || mainResp == nil {
			return nil, err
		}
		// Convert main response to transform response
		return &transform.Response{
			ID:         mainResp.ID,
			StatusCode: mainResp.StatusCode,
			Headers:    copyStringMap(mainResp.Headers),
			Body:       mainResp.Body,
			Error:      mainResp.Error,
			Metadata:   copyInterfaceMap(mainResp.Metadata),
			Timestamp:  mainResp.Timestamp,
			Duration:   mainResp.Duration,
		}, nil
	}

	// Call transform middleware
	transformResp, err := t.transformMiddleware.Process(ctx, transformReq, transformNext)

	// Convert transform response back to main response
	return ConvertFromTransformResponse(transformResp), err
}

func (t *TransformMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return t.transformMiddleware.Configure(config)
}

func (t *TransformMiddlewareAdapter) Enabled() bool {
	return t.transformMiddleware.Enabled()
}

func (t *TransformMiddlewareAdapter) Priority() int {
	return t.transformMiddleware.Priority()
}
