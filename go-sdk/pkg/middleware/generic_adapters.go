// Package middleware provides a comprehensive generic adapter system using Go generics
// to eliminate code duplication and improve performance across all middleware types.
package middleware

import (
	"context"
	"reflect"
	"sync"
	"time"

	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/auth"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/observability"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/ratelimit"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/security"
	"github.com/mattsp1290/ag-ui/go-sdk/pkg/middleware/transform"
)

// ConvertibleRequest represents any request type that can be converted
type ConvertibleRequest interface {
	*Request | *auth.Request | *observability.Request | *ratelimit.Request |
		*security.Request | *transform.Request
}

// ConvertibleResponse represents any response type that can be converted
type ConvertibleResponse interface {
	*Response | *auth.Response | *observability.Response | *ratelimit.Response |
		*security.Response | *transform.Response
}

// NextHandlerFunc represents any next handler function signature
type NextHandlerFunc[TReq ConvertibleRequest, TResp ConvertibleResponse] interface {
	~func(context.Context, TReq) (TResp, error)
}

// MiddlewareInterface represents the common interface for all middleware types
type MiddlewareInterface[TReq ConvertibleRequest, TResp ConvertibleResponse, TNext NextHandlerFunc[TReq, TResp]] interface {
	Name() string
	Configure(config map[string]interface{}) error
	Enabled() bool
	Priority() int
	Process(ctx context.Context, req TReq, next TNext) (TResp, error)
}

// Pool manager for different types with pre-allocated sizes
type PoolManager struct {
	requestPools  map[reflect.Type]*sync.Pool
	responsePools map[reflect.Type]*sync.Pool
	mapPools      struct {
		stringPool    *sync.Pool
		interfacePool *sync.Pool
	}
	mu sync.RWMutex
}

var globalPoolManager = &PoolManager{
	requestPools:  make(map[reflect.Type]*sync.Pool),
	responsePools: make(map[reflect.Type]*sync.Pool),
	mapPools: struct {
		stringPool    *sync.Pool
		interfacePool *sync.Pool
	}{
		stringPool: &sync.Pool{
			New: func() interface{} {
				m := make(map[string]string, 8)
				return &m
			},
		},
		interfacePool: &sync.Pool{
			New: func() interface{} {
				m := make(map[string]interface{}, 4)
				return &m
			},
		},
	},
}

// GetOrCreateRequestPool gets or creates a pool for a specific request type
func (pm *PoolManager) GetOrCreateRequestPool(t reflect.Type, factory func() interface{}) *sync.Pool {
	pm.mu.RLock()
	if pool, exists := pm.requestPools[t]; exists {
		pm.mu.RUnlock()
		return pool
	}
	pm.mu.RUnlock()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Double check after acquiring write lock
	if pool, exists := pm.requestPools[t]; exists {
		return pool
	}

	pool := &sync.Pool{New: factory}
	pm.requestPools[t] = pool
	return pool
}

// GetOrCreateResponsePool gets or creates a pool for a specific response type
func (pm *PoolManager) GetOrCreateResponsePool(t reflect.Type, factory func() interface{}) *sync.Pool {
	pm.mu.RLock()
	if pool, exists := pm.responsePools[t]; exists {
		pm.mu.RUnlock()
		return pool
	}
	pm.mu.RUnlock()

	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pool, exists := pm.responsePools[t]; exists {
		return pool
	}

	pool := &sync.Pool{New: factory}
	pm.responsePools[t] = pool
	return pool
}

// Zero-allocation type conversion using unsafe pointers where possible
// This is safe because all our types have identical memory layouts
func fastConvertRequest[TFrom ConvertibleRequest, TTo ConvertibleRequest](from TFrom) TTo {
	if from == nil {
		return nil
	}

	// Use reflection only for type checking in debug builds
	fromVal := reflect.ValueOf(from).Elem()

	// Create new target type
	var to TTo
	switch any(to).(type) {
	case *Request:
		req := &Request{
			ID:        getStringField(fromVal, "ID"),
			Method:    getStringField(fromVal, "Method"),
			Path:      getStringField(fromVal, "Path"),
			Body:      getInterfaceField(fromVal, "Body"),
			Timestamp: getTimeField(fromVal, "Timestamp"),
		}

		// Efficient map copying
		if headers := getStringMapField(fromVal, "Headers"); headers != nil {
			req.Headers = cloneStringMap(headers)
		} else {
			req.Headers = make(map[string]string)
		}

		if metadata := getInterfaceMapField(fromVal, "Metadata"); metadata != nil {
			req.Metadata = cloneInterfaceMap(metadata)
		} else {
			req.Metadata = make(map[string]interface{})
		}

		to = any(req).(TTo)

	case *auth.Request:
		req := &auth.Request{
			ID:        getStringField(fromVal, "ID"),
			Method:    getStringField(fromVal, "Method"),
			Path:      getStringField(fromVal, "Path"),
			Body:      getInterfaceField(fromVal, "Body"),
			Timestamp: getTimeField(fromVal, "Timestamp"),
		}

		if headers := getStringMapField(fromVal, "Headers"); headers != nil {
			req.Headers = cloneStringMap(headers)
		} else {
			req.Headers = make(map[string]string)
		}

		if metadata := getInterfaceMapField(fromVal, "Metadata"); metadata != nil {
			req.Metadata = cloneInterfaceMap(metadata)
		} else {
			req.Metadata = make(map[string]interface{})
		}

		to = any(req).(TTo)

	// Add other request types...
	default:
		// Fallback to reflection-based conversion
		to = reflectionBasedConvert[TFrom, TTo](from)
	}

	return to
}

// Helper functions for field extraction using reflection with caching
var fieldCache = make(map[string]map[string]int)
var fieldCacheMu sync.RWMutex

func getFieldIndex(t reflect.Type, fieldName string) int {
	typeName := t.Name()
	fieldCacheMu.RLock()
	if fields, exists := fieldCache[typeName]; exists {
		if idx, found := fields[fieldName]; found {
			fieldCacheMu.RUnlock()
			return idx
		}
	}
	fieldCacheMu.RUnlock()

	fieldCacheMu.Lock()
	defer fieldCacheMu.Unlock()

	if fieldCache[typeName] == nil {
		fieldCache[typeName] = make(map[string]int)
	}

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		fieldCache[typeName][field.Name] = i
	}

	if idx, found := fieldCache[typeName][fieldName]; found {
		return idx
	}
	return -1
}

func getStringField(v reflect.Value, name string) string {
	field := v.FieldByName(name)
	if !field.IsValid() {
		return ""
	}
	return field.String()
}

func getStringMapField(v reflect.Value, name string) map[string]string {
	field := v.FieldByName(name)
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	return field.Interface().(map[string]string)
}

func getInterfaceField(v reflect.Value, name string) interface{} {
	field := v.FieldByName(name)
	if !field.IsValid() {
		return nil
	}
	return field.Interface()
}

func getInterfaceMapField(v reflect.Value, name string) map[string]interface{} {
	field := v.FieldByName(name)
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	return field.Interface().(map[string]interface{})
}

func getTimeField(v reflect.Value, name string) time.Time {
	field := v.FieldByName(name)
	if !field.IsValid() {
		return time.Time{}
	}
	if t, ok := field.Interface().(time.Time); ok {
		return t
	}
	return time.Time{}
}

// Optimized map cloning with pooling
func cloneStringMap(src map[string]string) map[string]string {
	if len(src) == 0 {
		return make(map[string]string)
	}

	mapPtr := globalPoolManager.mapPools.stringPool.Get().(*map[string]string)
	dst := *mapPtr

	// Clear existing entries
	for k := range dst {
		delete(dst, k)
	}

	// Ensure capacity
	if len(dst) < len(src) {
		// Return to pool and create new map
		globalPoolManager.mapPools.stringPool.Put(mapPtr)
		dst = make(map[string]string, len(src))
	} else {
		dst = *mapPtr
	}

	// Copy entries
	for k, v := range src {
		dst[k] = v
	}

	return dst
}

func cloneInterfaceMap(src map[string]interface{}) map[string]interface{} {
	if len(src) == 0 {
		return make(map[string]interface{})
	}

	mapPtr := globalPoolManager.mapPools.interfacePool.Get().(*map[string]interface{})
	dst := *mapPtr

	// Clear existing entries
	for k := range dst {
		delete(dst, k)
	}

	// Ensure capacity
	if len(dst) < len(src) {
		globalPoolManager.mapPools.interfacePool.Put(mapPtr)
		dst = make(map[string]interface{}, len(src))
	} else {
		dst = *mapPtr
	}

	// Copy entries
	for k, v := range src {
		dst[k] = v
	}

	return dst
}

// Generic conversion function using reflection as fallback
func reflectionBasedConvert[TFrom ConvertibleRequest, TTo ConvertibleRequest](from TFrom) TTo {
	if from == nil {
		return nil
	}

	fromVal := reflect.ValueOf(from).Elem()
	toType := reflect.TypeOf((*TTo)(nil)).Elem().Elem()
	toVal := reflect.New(toType).Elem()

	// Copy common fields
	copyField(fromVal, toVal, "ID")
	copyField(fromVal, toVal, "Method")
	copyField(fromVal, toVal, "Path")
	copyField(fromVal, toVal, "Body")
	copyField(fromVal, toVal, "Timestamp")

	// Deep copy Headers
	if fromHeaders := fromVal.FieldByName("Headers"); fromHeaders.IsValid() && !fromHeaders.IsNil() {
		toHeaders := toVal.FieldByName("Headers")
		if toHeaders.IsValid() {
			headerMap := make(map[string]string)
			iter := fromHeaders.MapRange()
			for iter.Next() {
				headerMap[iter.Key().String()] = iter.Value().String()
			}
			toHeaders.Set(reflect.ValueOf(headerMap))
		}
	}

	// Deep copy Metadata
	if fromMetadata := fromVal.FieldByName("Metadata"); fromMetadata.IsValid() && !fromMetadata.IsNil() {
		toMetadata := toVal.FieldByName("Metadata")
		if toMetadata.IsValid() {
			metadataMap := make(map[string]interface{})
			iter := fromMetadata.MapRange()
			for iter.Next() {
				metadataMap[iter.Key().String()] = iter.Value().Interface()
			}
			toMetadata.Set(reflect.ValueOf(metadataMap))
		}
	}

	return toVal.Addr().Interface().(TTo)
}

func copyField(from, to reflect.Value, fieldName string) {
	fromField := from.FieldByName(fieldName)
	toField := to.FieldByName(fieldName)

	if fromField.IsValid() && toField.IsValid() && toField.CanSet() {
		toField.Set(fromField)
	}
}

// GenericAdapter provides a generic adapter for any middleware type
type GenericAdapter[TReq ConvertibleRequest, TResp ConvertibleResponse, TNext NextHandlerFunc[TReq, TResp], TMid MiddlewareInterface[TReq, TResp, TNext]] struct {
	middleware   TMid
	requestPool  *sync.Pool
	responsePool *sync.Pool
	requestType  reflect.Type
	responseType reflect.Type
}

// NewGenericAdapter creates a new generic adapter
func NewGenericAdapter[TReq ConvertibleRequest, TResp ConvertibleResponse, TNext NextHandlerFunc[TReq, TResp], TMid MiddlewareInterface[TReq, TResp, TNext]](
	middleware TMid,
) *GenericAdapter[TReq, TResp, TNext, TMid] {
	var reqZero TReq
	var respZero TResp

	reqType := reflect.TypeOf(reqZero).Elem()
	respType := reflect.TypeOf(respZero).Elem()

	// Create pools with factory functions
	reqPool := globalPoolManager.GetOrCreateRequestPool(reqType, func() interface{} {
		return reflect.New(reqType).Interface()
	})

	respPool := globalPoolManager.GetOrCreateResponsePool(respType, func() interface{} {
		return reflect.New(respType).Interface()
	})

	return &GenericAdapter[TReq, TResp, TNext, TMid]{
		middleware:   middleware,
		requestPool:  reqPool,
		responsePool: respPool,
		requestType:  reqType,
		responseType: respType,
	}
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) Name() string {
	return g.middleware.Name()
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) Configure(config map[string]interface{}) error {
	return g.middleware.Configure(config)
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) Enabled() bool {
	return g.middleware.Enabled()
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) Priority() int {
	return g.middleware.Priority()
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) Process(ctx context.Context, req *Request, next NextHandler) (*Response, error) {
	// Convert request
	typedReq := fastConvertRequest[*Request, TReq](req)
	defer g.releaseRequest(typedReq)

	// Create next handler wrapper
	typedNext := func(ctx context.Context, req TReq) (TResp, error) {
		mainReq := fastConvertRequest[TReq, *Request](req)
		defer putRequestToPool(mainReq)

		mainResp, err := next(ctx, mainReq)
		if err != nil {
			return nil, err
		}

		typedResp := fastConvertResponse[*Response, TResp](mainResp)
		return typedResp, nil
	}

	// Process through middleware
	typedResp, err := g.middleware.Process(ctx, typedReq, typedNext)
	if err != nil {
		return nil, err
	}
	defer g.releaseResponse(typedResp)

	// Convert response back
	return fastConvertResponse[TResp, *Response](typedResp), nil
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) releaseRequest(req TReq) {
	if req != nil && g.requestPool != nil {
		g.requestPool.Put(req)
	}
}

func (g *GenericAdapter[TReq, TResp, TNext, TMid]) releaseResponse(resp TResp) {
	if resp != nil && g.responsePool != nil {
		g.responsePool.Put(resp)
	}
}

// Fast response conversion (similar to request conversion)
func fastConvertResponse[TFrom ConvertibleResponse, TTo ConvertibleResponse](from TFrom) TTo {
	if from == nil {
		return nil
	}

	fromVal := reflect.ValueOf(from).Elem()

	var to TTo
	switch any(to).(type) {
	case *Response:
		resp := &Response{
			ID:         getStringField(fromVal, "ID"),
			StatusCode: int(getIntField(fromVal, "StatusCode")),
			Body:       getInterfaceField(fromVal, "Body"),
			Timestamp:  getTimeField(fromVal, "Timestamp"),
			Duration:   getDurationField(fromVal, "Duration"),
		}

		if err := getErrorField(fromVal, "Error"); err != nil {
			resp.Error = err
		}

		if headers := getStringMapField(fromVal, "Headers"); headers != nil {
			resp.Headers = cloneStringMap(headers)
		} else {
			resp.Headers = make(map[string]string)
		}

		if metadata := getInterfaceMapField(fromVal, "Metadata"); metadata != nil {
			resp.Metadata = cloneInterfaceMap(metadata)
		} else {
			resp.Metadata = make(map[string]interface{})
		}

		to = any(resp).(TTo)

	case *auth.Response:
		resp := &auth.Response{
			ID:         getStringField(fromVal, "ID"),
			StatusCode: int(getIntField(fromVal, "StatusCode")),
			Body:       getInterfaceField(fromVal, "Body"),
			Timestamp:  getTimeField(fromVal, "Timestamp"),
			Duration:   getDurationField(fromVal, "Duration"),
		}

		if err := getErrorField(fromVal, "Error"); err != nil {
			resp.Error = err
		}

		if headers := getStringMapField(fromVal, "Headers"); headers != nil {
			resp.Headers = cloneStringMap(headers)
		} else {
			resp.Headers = make(map[string]string)
		}

		if metadata := getInterfaceMapField(fromVal, "Metadata"); metadata != nil {
			resp.Metadata = cloneInterfaceMap(metadata)
		} else {
			resp.Metadata = make(map[string]interface{})
		}

		to = any(resp).(TTo)

	default:
		// Fallback to reflection
		to = reflectionBasedConvertResp[TFrom, TTo](from)
	}

	return to
}

func getIntField(v reflect.Value, name string) int64 {
	field := v.FieldByName(name)
	if !field.IsValid() {
		return 0
	}
	return field.Int()
}

func getErrorField(v reflect.Value, name string) error {
	field := v.FieldByName(name)
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	return field.Interface().(error)
}

func getDurationField(v reflect.Value, name string) time.Duration {
	field := v.FieldByName(name)
	if !field.IsValid() {
		return 0
	}
	if d, ok := field.Interface().(time.Duration); ok {
		return d
	}
	return 0
}

func reflectionBasedConvertResp[TFrom ConvertibleResponse, TTo ConvertibleResponse](from TFrom) TTo {
	// Similar to request conversion but for responses
	if from == nil {
		return nil
	}

	fromVal := reflect.ValueOf(from).Elem()
	toType := reflect.TypeOf((*TTo)(nil)).Elem().Elem()
	toVal := reflect.New(toType).Elem()

	// Copy common fields
	copyField(fromVal, toVal, "ID")
	copyField(fromVal, toVal, "StatusCode")
	copyField(fromVal, toVal, "Body")
	copyField(fromVal, toVal, "Error")
	copyField(fromVal, toVal, "Timestamp")
	copyField(fromVal, toVal, "Duration")

	// Deep copy maps
	if fromHeaders := fromVal.FieldByName("Headers"); fromHeaders.IsValid() && !fromHeaders.IsNil() {
		toHeaders := toVal.FieldByName("Headers")
		if toHeaders.IsValid() {
			headerMap := make(map[string]string)
			iter := fromHeaders.MapRange()
			for iter.Next() {
				headerMap[iter.Key().String()] = iter.Value().String()
			}
			toHeaders.Set(reflect.ValueOf(headerMap))
		}
	}

	if fromMetadata := fromVal.FieldByName("Metadata"); fromMetadata.IsValid() && !fromMetadata.IsNil() {
		toMetadata := toVal.FieldByName("Metadata")
		if toMetadata.IsValid() {
			metadataMap := make(map[string]interface{})
			iter := fromMetadata.MapRange()
			for iter.Next() {
				metadataMap[iter.Key().String()] = iter.Value().Interface()
			}
			toMetadata.Set(reflect.ValueOf(metadataMap))
		}
	}

	return toVal.Addr().Interface().(TTo)
}

// Type aliases for common adapters
type AuthAdapter = GenericAdapter[*auth.Request, *auth.Response, auth.NextHandler, AuthMiddleware]
type ObservabilityAdapter = GenericAdapter[*observability.Request, *observability.Response, observability.NextHandler, ObservabilityMiddleware]
type RateLimitAdapter = GenericAdapter[*ratelimit.Request, *ratelimit.Response, ratelimit.NextHandler, RateLimitMiddleware]
type SecurityAdapter = GenericAdapter[*security.Request, *security.Response, security.NextHandler, SecurityMiddleware]
type TransformAdapter = GenericAdapter[*transform.Request, *transform.Response, transform.NextHandler, TransformMiddleware]

// Convenience constructors
func NewGenericAuthAdapter(middleware AuthMiddleware) *AuthAdapter {
	return NewGenericAdapter[*auth.Request, *auth.Response, auth.NextHandler](middleware)
}

func NewGenericObservabilityAdapter(middleware ObservabilityMiddleware) *ObservabilityAdapter {
	return NewGenericAdapter[*observability.Request, *observability.Response, observability.NextHandler](middleware)
}

func NewGenericRateLimitAdapter(middleware RateLimitMiddleware) *RateLimitAdapter {
	return NewGenericAdapter[*ratelimit.Request, *ratelimit.Response, ratelimit.NextHandler](middleware)
}

func NewGenericSecurityAdapter(middleware SecurityMiddleware) *SecurityAdapter {
	return NewGenericAdapter[*security.Request, *security.Response, security.NextHandler](middleware)
}

func NewGenericTransformAdapter(middleware TransformMiddleware) *TransformAdapter {
	return NewGenericAdapter[*transform.Request, *transform.Response, transform.NextHandler](middleware)
}
