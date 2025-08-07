// Package middleware provides optimized adapter implementations using Go generics
// and object pooling to reduce memory allocation overhead and code duplication.
package middleware

import (
	"context"
	"sync"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/observability"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/ratelimit"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/transform"
)

// Generic type constraints for middleware types
type MiddlewareRequest interface {
	*Request | *auth.Request | *observability.Request | *ratelimit.Request | *security.Request | *transform.Request
}

type MiddlewareResponse interface {
	*Response | *auth.Response | *observability.Response | *ratelimit.Response | *security.Response | *transform.Response
}

// RequestInterface defines common methods for all request types
type RequestInterface interface {
	GetID() string
	GetMethod() string
	GetPath() string
	GetHeaders() map[string]string
	GetBody() interface{}
	GetMetadata() map[string]interface{}
	GetTimestamp() any
}

// ResponseInterface defines common methods for all response types
type ResponseInterface interface {
	GetID() string
	GetStatusCode() int
	GetHeaders() map[string]string
	GetBody() interface{}
	GetError() error
	GetMetadata() map[string]interface{}
	GetTimestamp() any
	GetDuration() any
}

// Object pools for Request/Response reuse
var (
	requestPool = sync.Pool{
		New: func() interface{} {
			return &Request{
				Headers:  make(map[string]string, 8), // Pre-allocate common size
				Metadata: make(map[string]interface{}, 4),
			}
		},
	}

	responsePool = sync.Pool{
		New: func() interface{} {
			return &Response{
				Headers:  make(map[string]string, 8), // Pre-allocate common size
				Metadata: make(map[string]interface{}, 4),
			}
		},
	}

	authRequestPool = sync.Pool{
		New: func() interface{} {
			return &auth.Request{
				Headers:  make(map[string]string, 8),
				Metadata: make(map[string]interface{}, 4),
			}
		},
	}

	authResponsePool = sync.Pool{
		New: func() interface{} {
			return &auth.Response{
				Headers:  make(map[string]string, 8),
				Metadata: make(map[string]interface{}, 4),
			}
		},
	}

	stringMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]string, 8)
		},
	}

	interfaceMapPool = sync.Pool{
		New: func() interface{} {
			return make(map[string]interface{}, 4)
		},
	}
)

// Optimized map copying functions using pools
func copyStringMapOptimized(src map[string]string) map[string]string {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return make(map[string]string)
	}

	// Get from pool and reset
	dst := stringMapPool.Get().(map[string]string)
	for k := range dst {
		delete(dst, k)
	}

	// Ensure capacity
	if len(dst) < len(src) {
		// Return to pool and create new map with correct size
		stringMapPool.Put(dst)
		dst = make(map[string]string, len(src))
	}

	// Copy data
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

func copyInterfaceMapOptimized(src map[string]interface{}) map[string]interface{} {
	if src == nil {
		return nil
	}
	if len(src) == 0 {
		return make(map[string]interface{})
	}

	// Get from pool and reset
	dst := interfaceMapPool.Get().(map[string]interface{})
	for k := range dst {
		delete(dst, k)
	}

	// Ensure capacity
	if len(dst) < len(src) {
		// Return to pool and create new map with correct size
		interfaceMapPool.Put(dst)
		dst = make(map[string]interface{}, len(src))
	}

	// Copy data
	for k, v := range src {
		dst[k] = v
	}
	return dst
}

// Generic conversion functions using type parameters
func convertRequestGeneric[T MiddlewareRequest](src *Request, pool *sync.Pool) T {
	if src == nil {
		return nil
	}

	var target T
	if pool != nil {
		target = pool.Get().(T)
		// Reset the target object
		switch req := any(target).(type) {
		case *Request:
			req.ID = ""
			req.Method = ""
			req.Path = ""
			req.Body = nil
			req.Timestamp = src.Timestamp
			// Clear maps efficiently
			for k := range req.Headers {
				delete(req.Headers, k)
			}
			for k := range req.Metadata {
				delete(req.Metadata, k)
			}
		case *auth.Request:
			req.ID = ""
			req.Method = ""
			req.Path = ""
			req.Body = nil
			req.Timestamp = src.Timestamp
			for k := range req.Headers {
				delete(req.Headers, k)
			}
			for k := range req.Metadata {
				delete(req.Metadata, k)
			}
			// Add other types as needed
		}
	} else {
		// Create new instance - this will be type-switched
		switch any(target).(type) {
		case *Request:
			target = any(&Request{
				Headers:  make(map[string]string, len(src.Headers)),
				Metadata: make(map[string]interface{}, len(src.Metadata)),
			}).(T)
		case *auth.Request:
			target = any(&auth.Request{
				Headers:  make(map[string]string, len(src.Headers)),
				Metadata: make(map[string]interface{}, len(src.Metadata)),
			}).(T)
			// Add other types as needed
		}
	}

	// Copy fields efficiently using type assertion
	switch req := any(target).(type) {
	case *Request:
		req.ID = src.ID
		req.Method = src.Method
		req.Path = src.Path
		req.Body = src.Body
		req.Timestamp = src.Timestamp
		copyMapInPlace(req.Headers, src.Headers)
		copyInterfaceMapInPlace(req.Metadata, src.Metadata)
	case *auth.Request:
		req.ID = src.ID
		req.Method = src.Method
		req.Path = src.Path
		req.Body = src.Body
		req.Timestamp = src.Timestamp
		copyMapInPlace(req.Headers, src.Headers)
		copyInterfaceMapInPlace(req.Metadata, src.Metadata)
		// Add other request types...
	}

	return target
}

// Helper functions for in-place copying
func copyMapInPlace(dst, src map[string]string) {
	for k, v := range src {
		dst[k] = v
	}
}

func copyInterfaceMapInPlace(dst map[string]interface{}, src map[string]interface{}) {
	for k, v := range src {
		dst[k] = v
	}
}

// Pool management functions
func putRequestToPool(req *Request) {
	if req == nil {
		return
	}

	// Clear sensitive data
	req.Body = nil

	// Only return to pool if maps aren't too large (prevent memory leaks)
	if len(req.Headers) <= 16 && len(req.Metadata) <= 8 {
		requestPool.Put(req)
	}
}

func putResponseToPool(resp *Response) {
	if resp == nil {
		return
	}

	// Clear sensitive data
	resp.Body = nil
	resp.Error = nil

	// Only return to pool if maps aren't too large
	if len(resp.Headers) <= 16 && len(resp.Metadata) <= 8 {
		responsePool.Put(resp)
	}
}

func putAuthRequestToPool(req *auth.Request) {
	if req == nil {
		return
	}

	req.Body = nil
	if len(req.Headers) <= 16 && len(req.Metadata) <= 8 {
		authRequestPool.Put(req)
	}
}

func putAuthResponseToPool(resp *auth.Response) {
	if resp == nil {
		return
	}

	resp.Body = nil
	resp.Error = nil
	if len(resp.Headers) <= 16 && len(resp.Metadata) <= 8 {
		authResponsePool.Put(resp)
	}
}

// Optimized conversion functions for auth package
func ConvertToAuthRequestOptimized(req *Request) *auth.Request {
	if req == nil {
		return nil
	}

	authReq := authRequestPool.Get().(*auth.Request)

	// Clear existing data
	authReq.ID = ""
	authReq.Method = ""
	authReq.Path = ""
	authReq.Body = nil
	for k := range authReq.Headers {
		delete(authReq.Headers, k)
	}
	for k := range authReq.Metadata {
		delete(authReq.Metadata, k)
	}

	// Copy new data
	authReq.ID = req.ID
	authReq.Method = req.Method
	authReq.Path = req.Path
	authReq.Body = req.Body
	authReq.Timestamp = req.Timestamp

	copyMapInPlace(authReq.Headers, req.Headers)
	copyInterfaceMapInPlace(authReq.Metadata, req.Metadata)

	return authReq
}

func ConvertFromAuthRequestOptimized(req *auth.Request) *Request {
	if req == nil {
		return nil
	}

	mainReq := requestPool.Get().(*Request)

	// Clear existing data
	mainReq.ID = ""
	mainReq.Method = ""
	mainReq.Path = ""
	mainReq.Body = nil
	for k := range mainReq.Headers {
		delete(mainReq.Headers, k)
	}
	for k := range mainReq.Metadata {
		delete(mainReq.Metadata, k)
	}

	// Copy new data
	mainReq.ID = req.ID
	mainReq.Method = req.Method
	mainReq.Path = req.Path
	mainReq.Body = req.Body
	mainReq.Timestamp = req.Timestamp

	copyMapInPlace(mainReq.Headers, req.Headers)
	copyInterfaceMapInPlace(mainReq.Metadata, req.Metadata)

	return mainReq
}

func ConvertToAuthResponseOptimized(resp *Response) *auth.Response {
	if resp == nil {
		return nil
	}

	authResp := authResponsePool.Get().(*auth.Response)

	// Clear and copy
	authResp.ID = resp.ID
	authResp.StatusCode = resp.StatusCode
	authResp.Body = resp.Body
	authResp.Error = resp.Error
	authResp.Timestamp = resp.Timestamp
	authResp.Duration = resp.Duration

	// Clear maps
	for k := range authResp.Headers {
		delete(authResp.Headers, k)
	}
	for k := range authResp.Metadata {
		delete(authResp.Metadata, k)
	}

	copyMapInPlace(authResp.Headers, resp.Headers)
	copyInterfaceMapInPlace(authResp.Metadata, resp.Metadata)

	return authResp
}

func ConvertFromAuthResponseOptimized(resp *auth.Response) *Response {
	if resp == nil {
		return nil
	}

	mainResp := responsePool.Get().(*Response)

	// Clear and copy
	mainResp.ID = resp.ID
	mainResp.StatusCode = resp.StatusCode
	mainResp.Body = resp.Body
	mainResp.Error = resp.Error
	mainResp.Timestamp = resp.Timestamp
	mainResp.Duration = resp.Duration

	// Clear maps
	for k := range mainResp.Headers {
		delete(mainResp.Headers, k)
	}
	for k := range mainResp.Metadata {
		delete(mainResp.Metadata, k)
	}

	copyMapInPlace(mainResp.Headers, resp.Headers)
	copyInterfaceMapInPlace(mainResp.Metadata, resp.Metadata)

	return mainResp
}

// Generic middleware adapter using Go generics
type GenericMiddlewareAdapter[TReq MiddlewareRequest, TResp MiddlewareResponse, TMiddleware any] struct {
	middleware      TMiddleware
	convertToReq    func(*Request) TReq
	convertFromReq  func(TReq) *Request
	convertToResp   func(*Response) TResp
	convertFromResp func(TResp) *Response
	releaseReq      func(TReq)
	releaseResp     func(TResp)

	// Cached method accessors for performance
	getName     func(TMiddleware) string
	process     func(TMiddleware, context.Context, TReq, interface{}) (TResp, error)
	configure   func(TMiddleware, map[string]interface{}) error
	getEnabled  func(TMiddleware) bool
	getPriority func(TMiddleware) int
}

// NewGenericMiddlewareAdapter creates a new generic middleware adapter
func NewGenericMiddlewareAdapter[TReq MiddlewareRequest, TResp MiddlewareResponse, TMiddleware any](
	middleware TMiddleware,
	convertToReq func(*Request) TReq,
	convertFromReq func(TReq) *Request,
	convertToResp func(*Response) TResp,
	convertFromResp func(TResp) *Response,
	releaseReq func(TReq),
	releaseResp func(TResp),
) *GenericMiddlewareAdapter[TReq, TResp, TMiddleware] {
	return &GenericMiddlewareAdapter[TReq, TResp, TMiddleware]{
		middleware:      middleware,
		convertToReq:    convertToReq,
		convertFromReq:  convertFromReq,
		convertToResp:   convertToResp,
		convertFromResp: convertFromResp,
		releaseReq:      releaseReq,
		releaseResp:     releaseResp,
	}
}

// OptimizedAuthMiddlewareAdapter - optimized version using pools
type OptimizedAuthMiddlewareAdapter struct {
	authMiddleware AuthMiddleware
}

func NewOptimizedAuthMiddlewareAdapter(authMiddleware AuthMiddleware) *OptimizedAuthMiddlewareAdapter {
	return &OptimizedAuthMiddlewareAdapter{
		authMiddleware: authMiddleware,
	}
}

func (a *OptimizedAuthMiddlewareAdapter) Name() string {
	return a.authMiddleware.Name()
}

func (a *OptimizedAuthMiddlewareAdapter) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert using optimized pooled functions
	authReq := ConvertToAuthRequestOptimized(req)
	defer putAuthRequestToPool(authReq)

	// Create optimized auth NextHandler wrapper
	authNext := func(ctx context.Context, authReq *auth.Request) (*auth.Response, error) {
		mainReq := ConvertFromAuthRequestOptimized(authReq)
		defer putRequestToPool(mainReq)

		mainResp, err := next(ctx, mainReq)
		if err != nil {
			return nil, err
		}

		authResp := ConvertToAuthResponseOptimized(mainResp)
		return authResp, nil
	}

	// Call auth middleware
	authResp, err := a.authMiddleware.Process(ctx, authReq, authNext)
	if err != nil {
		return nil, err
	}

	defer putAuthResponseToPool(authResp)

	// Convert auth response back to main response
	return ConvertFromAuthResponseOptimized(authResp), nil
}

func (a *OptimizedAuthMiddlewareAdapter) Configure(config map[string]interface{}) error {
	return a.authMiddleware.Configure(config)
}

func (a *OptimizedAuthMiddlewareAdapter) Enabled() bool {
	return a.authMiddleware.Enabled()
}

func (a *OptimizedAuthMiddlewareAdapter) Priority() int {
	return a.authMiddleware.Priority()
}
